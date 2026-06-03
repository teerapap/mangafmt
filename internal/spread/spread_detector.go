// Package spread decides whether two manga page images form a two-page
// spread — a single illustration printed across facing pages.
//
// Algorithm
//
//  1. Pre-filter pages that are unlikely to be part of a spread. Each
//     trigger applies a multiplicative penalty to the final score:
//     - "Boxy" pages: internal gutters — low-variance rows OR columns
//     away from the page edges — indicating a multi-panel layout
//     (stacked, side-by-side, or grid).
//     - Binding-edge margin mismatch. A spread needs the two seams to
//     agree across the page boundary: either both sides have content
//     flowing in, or both sides share a flat background of the same
//     color. Anything else — solid margins of different colors, or one
//     side solid while the other has content — indicates two
//     independent pages and is penalised.
//
//  2. Search for the best vertical-shift alignment in [-MaxVerticalShift,
//     +MaxVerticalShift]. Catches scan-registration errors so the
//     per-row heuristics aren't penalised for a small vertical offset.
//
//  3. Compute four similarity heuristics at the seam (right edge of `left`
//     joined to left edge of `right`). All produce a score in [0,1]:
//     - Edge RMSE             color RMSE of K-column strips
//     - Histogram similarity  3D RGB Bhattacharyya coefficient
//     - Gradient match        correlation of vertical brightness derivatives
//     - Edge alignment        Pearson correlation of Sobel edge magnitudes
//     at the boundary column of each page
//
//  4. Combine with weights that sum to 1.0, apply penalties, threshold.
//
// Color and grayscale values are computed on demand via image.Image.At —
// no pixel buffers are allocated; all accesses go through the original image.
//
// Usage
//
//	sd := spread.NewSpreadDetector()
//	res, err := sd.IsTwoPageSpread(left, right)
//	if err != nil { /* ... */ }
//	fmt.Printf("spread=%v shift=%d confidence=%.2f (%s)\n",
//	    res.IsSpread, res.VerticalShift, res.Confidence, res.Reason)
package spread

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"
)

// ----------------------------- tuning ------------------------------------ //

// SpreadDetector bundles every tuning parameter for spread detection and
// exposes IsTwoPageSpread as a method. Obtain a fully-populated value
// from NewSpreadDetector and override individual fields as needed.
type SpreadDetector struct {
	// --- Strip widths ---

	EdgeStripWidth int     // columns averaged on each side for edge RMSE
	SeamStripFrac  float64 // wider seam for histogram / gradient
	MinSeamStrip   int     // floor for very small images

	// MaxVerticalShift is the vertical-shift search range (rows). The
	// search runs over [-MaxVerticalShift, +MaxVerticalShift], so
	// width = 2N+1 trials.
	MaxVerticalShift int

	// --- "Boxy" page detection ---
	//
	// A gutter is a band of consecutive rows OR columns whose brightness
	// variance is below GutterVarianceMax — a flat strip of any tone
	// (white, black, gray). A page is boxy if any of these holds:
	//   1. ≥ BoxyHGutterMin horizontal gutters span the full page width
	//      (stacked-panel layout).
	//   2. ≥ BoxyVGutterMin vertical gutters span the full page height
	//      (side-by-side panels).
	//   3. ≥1 horizontal gutter splits the page into bands, and at least
	//      one band has ≥1 internal vertical gutter (grid / mixed
	//      layout — manga pages typically divide horizontally first and
	//      then split each band into panels with vertical gutters that
	//      DO NOT extend through the full page height). The symmetric
	//      case (vertical-first split) is also checked.
	//
	// A single horizontal or vertical gutter alone is intentionally NOT
	// boxy — could be a chapter-title band or a tall figure on a flat
	// background.
	GutterVarianceMax  float64
	GutterMinThickness int
	// GutterMaxThicknessFrac caps gutter thickness. A genuine panel
	// gutter is a THIN divider between panels. A low-variance band
	// thicker than this fraction of the scan axis is a flat region of
	// the artwork itself — a uniform sky, a ship's hull, a calm stretch
	// of sea — not a panel boundary, and is not counted as a gutter.
	// Without this cap a large flat band in a single full-page
	// illustration reads as a gutter and the page is wrongly flagged
	// boxy.
	GutterMaxThicknessFrac float64
	// BoxyMinPanelExtentFrac sets the minimum panel-band size. A gutter
	// only divides two panels if there is a substantial panel band on
	// BOTH sides of it. A low-variance run pressed against the page
	// edge, or against another gutter with only a sliver between them,
	// is texture or a chapter-title block misread as a divider, not a
	// panel boundary. A band shorter than this fraction of the scan
	// axis does not count as a panel.
	BoxyMinPanelExtentFrac float64
	BoxyHGutterMin         int
	BoxyVGutterMin         int
	// BoxyMinBandExtent is the minimum extent (in pixels along its
	// axis) of a panel band to bother scanning for in-band gutters.
	// Skips slivers (e.g., a 20-px chapter title band) that can't host
	// a real panel layout.
	BoxyMinBandExtent int

	// --- Neighbor-contrast filter for findGutters ---
	//
	// A low-variance band is only a real panel gutter if it sits
	// BETWEEN two regions that are markedly more textured than the band
	// itself. Flat-toned art (a uniform halftone sky, a uniform dark
	// sea) produces low-variance rows that look identical to a panel
	// gutter under the variance-only rule, but those flat regions are
	// surrounded by more of the same flatness — not by panel content.
	// We compute the variance of the GutterNeighborWindow rows/columns
	// on each side of the candidate and require the LESS-textured side
	// to be at least GutterNeighborContrastMin × the band's own
	// variance. Min, not max, because a real gutter has content on BOTH
	// sides; "high variance on one side, flat on the other" describes a
	// content-to-flat transition, not a divider.
	//
	// Threshold rationale (from probing the test corpus): real panel
	// gutters score ratios in the hundreds to thousands (515+); false
	// gutters in flat-toned illustrations top out around 65. 100 sits
	// between the distributions with room to spare on both sides.
	GutterNeighborWindow      int
	GutterNeighborContrastMin float64
	// GutterBandVarianceFloor floors the band's variance to keep the
	// ratio finite for pixel-perfectly-flat gutters (variance ~ 0).
	GutterBandVarianceFloor float64

	// --- Solid-color margin detection (any color, not just white) ---

	MarginFrac       float64
	MarginChannelTol float64
	// MarginMatchTol is the per-channel tolerance for deciding whether
	// two solid binding-edge margins (one from each page) are the SAME
	// color. If they match, it's a shared illustration background
	// flowing across the seam, not two independent page margins meeting
	// at the spine — and the MarginMismatchPenalty is skipped
	// accordingly.
	MarginMatchTol float64

	// SeamInfoVarianceFull is the seam-informativeness threshold:
	// brightness variance (over the full seam strip) above which a side
	// is treated as fully informative. Below it, the similarity-based
	// heuristics (EdgeContinuity, HistSimilarity) are scaled down
	// proportionally — they fire on trivial matches when both seams are
	// uniform (white margin, black margin, any flat background), and
	// that match carries no information.
	SeamInfoVarianceFull float64

	// ColorHistBinsPerChan sets the color histogram resolution.
	ColorHistBinsPerChan int // 4 → 4^3 = 64 bins total

	// --- Weights (must sum to 1.0) ---

	WEdgeRMSE  float64
	WHistSim   float64
	WGradMatch float64
	WEdgeAlign float64

	// --- Decision ---

	SpreadThreshold float64
	BoxyPenalty     float64
	// MarginMismatchPenalty fires when the two binding edges don't
	// agree across the seam — either (a) both pages have solid-color
	// margins of DIFFERENT colors (independent framing), or (b) only
	// ONE page has a solid margin while the other has content right up
	// to the seam (asymmetric: content can't flow into a flat strip).
	// Both sub-cases are evidence of two independent pages, not a
	// spread.
	MarginMismatchPenalty float64
	EdgeRMSENormalizer    float64

	// --- Row-wise alignment gate ---
	//
	// A real spread MUST have something — brightness pattern, edge
	// structure — actually continuing across the seam. That can only
	// show up as a positive correlation between the two seam profiles,
	// which is what the three row-wise heuristics (GradientMatch,
	// EdgeAlignment, SeamBrightnessCorrelation) measure. EdgeContinuity
	// and HistSimilarity, by contrast, are global statistics: two
	// unrelated manga pages with the typical mostly-white content near
	// the seam will score ~1.0 on HistSimilarity and high on
	// EdgeContinuity regardless of any actual continuity.
	//
	// All three correlation scores are remapped from Pearson c ∈ [-1, 1]
	// to (c+1)/2 ∈ [0, 1], so 0.5 means "no correlation" (chance
	// baseline). The gate fires only when ALL THREE fail their
	// respective minima:
	//   - GradientMatch and EdgeAlignment are HIGH-frequency signals
	//     (derivative of brightness; Sobel magnitude). They're specific
	//     but noisy — real spreads with slight registration drift or
	//     unmatched inking density can produce near-chance scores here
	//     even when broad alignment exists. Lower threshold: c ≥ 0.2.
	//   - SeamBrightnessCorrelation is the LOW-frequency signal (raw
	//     mean brightness per row, no derivative). It captures broad
	//     illumination/pattern continuity that the derivative misses,
	//     and is the most robust evidence that a real spread exists.
	//     Higher threshold: c ≥ 0.4 — pairs of unrelated manga pages
	//     can score moderately positive here just from shared layout
	//     conventions (dark band top, white middle, dark band bottom),
	//     so we require a stronger signal to count it as evidence.
	//
	// Only-one-below is intentionally NOT penalized: real spreads can
	// have asymmetric profiles (e.g., broad brightness aligns strongly
	// but Sobel edges disagree because one side is screentone and the
	// other is line art).
	CorrelationGateMin float64
	BrightCorrGateMin  float64
	NoAlignmentPenalty float64

	// --- Strong seam continuity ---
	//
	// When the broad brightness pattern AND the structural edges both
	// clearly continue across the seam, a real illustration is
	// demonstrably flowing across the binding. An asymmetric-margin
	// pre-filter hit is then a false alarm (a thick panel-border line
	// misread as a page margin) and its penalty is suppressed. Both
	// conditions are required: a high brightness correlation alone can
	// arise from shared layout conventions, but combined with aligned
	// Sobel edges it is specific to real content crossing the seam.
	StrongContinuityBright float64
	StrongContinuityEdge   float64

	// --- Edge-only alignment penalty ---
	//
	// EdgeAlignment (Sobel) and GradientMatch are high-frequency signals
	// that lock onto the conventional black panel-border line at the
	// binding edge. Two independent pages framed the same way produce a
	// near-perfect Sobel correlation while their actual tonal content
	// does not continue. A real spread always carries the broad
	// brightness pattern across the seam too, so EdgeAlignment never
	// runs far ahead of SeamBrightnessCorrelation. When the gap exceeds
	// EdgeBrightDivergenceMax, the structural "match" is a framing
	// artifact and is penalised.
	EdgeBrightDivergenceMax float64
	EdgeOnlyPenalty         float64

	// InfoFloor is the blank-seam veto. Below this seam informativeness,
	// both seam strips are a uniform tone and there is no information
	// at the binding to decide a spread either way — whatever the
	// heuristics produced on near-constant signals is noise. The pair
	// is forced to not-a-spread.
	InfoFloor float64

	// --- Shared flat-background spread ---
	//
	// A sparse illustration on a uniform field (a night sky, a black
	// void) printed across both pages joins at the binding through the
	// flat field itself, not through content, so the row-wise
	// correlations sit at chance and cannot vouch for it. Instead it is
	// recognised when both binding edges are solid, their colors match,
	// the seam strips are near-identical (EdgeContinuity very high),
	// the seam carries a little variation but not much (flat, yet not
	// blank), and — the key guard against mistaking two ordinary
	// white-margined pages for a spread — the solid color reaches deep
	// into both pages instead of being a thin page margin.
	FlatBgEdgeContinuityMin float64
	FlatBgInfoMax           float64
	FlatBgDepthFrac         float64
	FlatBgDepthMin          float64
}

// NewSpreadDetector returns a SpreadDetector with tuning parameters that
// work well for typical 1000–2000 px wide manga scans. Override individual
// fields as needed for your data before calling IsTwoPageSpread.
func NewSpreadDetector() SpreadDetector {
	return SpreadDetector{
		EdgeStripWidth: 30,
		SeamStripFrac:  0.04,
		MinSeamStrip:   8,

		MaxVerticalShift: 3,

		GutterVarianceMax:      50.0,
		GutterMinThickness:     3,
		GutterMaxThicknessFrac: 0.12,
		BoxyMinPanelExtentFrac: 0.10,
		BoxyHGutterMin:         2,
		BoxyVGutterMin:         2,
		BoxyMinBandExtent:      60,

		GutterNeighborWindow:      30,
		GutterNeighborContrastMin: 100.0,
		GutterBandVarianceFloor:   1.0,

		MarginFrac:       0.85,
		MarginChannelTol: 20.0,
		MarginMatchTol:   20.0,

		SeamInfoVarianceFull: 1000.0,

		ColorHistBinsPerChan: 4,

		WEdgeRMSE:  0.30,
		WHistSim:   0.15,
		WGradMatch: 0.15,
		WEdgeAlign: 0.40,

		SpreadThreshold:       0.55,
		BoxyPenalty:           0.60,
		MarginMismatchPenalty: 0.60,
		EdgeRMSENormalizer:    80.0,

		CorrelationGateMin: 0.60,
		BrightCorrGateMin:  0.70,
		NoAlignmentPenalty: 0.55,

		StrongContinuityBright: 0.78,
		StrongContinuityEdge:   0.66,

		EdgeBrightDivergenceMax: 0.25,
		EdgeOnlyPenalty:         0.60,

		InfoFloor: 0.10,

		FlatBgEdgeContinuityMin: 0.85,
		FlatBgInfoMax:           0.40,
		FlatBgDepthFrac:         0.25,
		FlatBgDepthMin:          0.70,
	}
}

// ----------------------------- public API ------------------------------- //

// Result is the full breakdown of a spread-detection run.
type Result struct {
	IsSpread   bool
	Confidence float64 // final [0,1] score, after penalties
	Reason     string

	// VerticalShift is the row offset (in pixels of the right page) chosen
	// to maximise seam alignment. A non-zero value indicates the scans are
	// not perfectly registered; |shift| at the edge of the search range
	// (MaxVerticalShift) suggests the true offset may be larger.
	VerticalShift int

	EdgeContinuity            float64 // 1 = strips match perfectly, 0 = very different
	HistSimilarity            float64 // 1 = identical color distributions, 0 = disjoint
	GradientMatch             float64 // 1 = same vertical pattern, 0 = anti-correlated
	EdgeAlignment             float64 // 1 = matching structural edges row-by-row
	SeamBrightnessCorrelation float64 // 1 = same broad brightness pattern, 0.5 = uncorrelated

	// SeamInformativeness reflects how much variation each seam strip
	// contains. Near 0 when both sides are uniform (white margin, black
	// margin, flat tone) — in that regime, similarity-based heuristics
	// (EdgeContinuity, HistSimilarity) match trivially and
	// are scaled down by this factor in the final score. The per-heuristic
	// fields above are reported BEFORE scaling, so you can see both the
	// raw similarity and whether it carried any information.
	SeamInformativeness float64

	LeftBoxy       bool // left page looks like a multi-panel layout
	RightBoxy      bool // right page looks like a multi-panel layout
	LeftHasMargin  bool // left binding edge is a solid-color page margin
	RightHasMargin bool // right binding edge is a solid-color page margin
}

// IsTwoPageSpread analyses a pair of manga pages (`left` first, `right`
// second) and reports whether they form a two-page spread. Heights may
// differ — coordinates are sampled proportionally.
func (sd *SpreadDetector) IsTwoPageSpread(left, right image.Image) (Result, error) {
	var res Result
	if left == nil || right == nil {
		return res, fmt.Errorf("nil image")
	}
	if left.Bounds().Empty() || right.Bounds().Empty() {
		return res, fmt.Errorf("empty image")
	}

	lv := newView(left)
	rv := newView(right)

	// Step 1 — quick rule-outs.
	res.LeftBoxy = sd.isBoxy(lv)
	res.RightBoxy = sd.isBoxy(rv)
	leftSolid, lMR, lMG, lMB := sd.hasSolidColorMargin(lv, true)
	rightSolid, rMR, rMG, rMB := sd.hasSolidColorMargin(rv, false)
	res.LeftHasMargin = leftSolid
	res.RightHasMargin = rightSolid

	// Step 2 — find best vertical-shift alignment.
	res.VerticalShift = sd.findBestVerticalShift(lv, rv, sd.MaxVerticalShift)
	sh := res.VerticalShift

	// Step 3 — seam heuristics (those marked /shift-aware/ use sh).
	res.EdgeContinuity = sd.edgeContinuityScore(lv, rv, sh)        // shift-aware
	res.HistSimilarity = sd.histogramSimilarityScore(lv, rv)       // shift-independent
	res.GradientMatch = sd.gradientMatchScore(lv, rv, sh)          // shift-aware
	res.EdgeAlignment = horizontalEdgeAlignmentScore(lv, rv, sh)   // shift-aware
	res.SeamBrightnessCorrelation = sd.seamBrightnessCorrelationScore(lv, rv, sh)

	// Step 4 — seam informativeness. The similarity heuristics
	// (EdgeContinuity, HistSimilarity) match trivially when
	// both seam strips are uniform — solid white margin, solid black
	// margin, flat tone, anything constant. Gate them on whether either
	// side actually has variation to compare. `max` (not `min`) is
	// deliberate: if one side has content, similarity is naturally low,
	// so no gating is needed; only the both-uniform case is dangerous.
	infoL := clamp01(sd.seamStripBrightnessVariance(lv, true) / sd.SeamInfoVarianceFull)
	infoR := clamp01(sd.seamStripBrightnessVariance(rv, false) / sd.SeamInfoVarianceFull)
	info := math.Max(infoL, infoR)
	res.SeamInformativeness = info

	// Shared flat-background spread — see the FlatBg* fields. The solid
	// color must also extend deep into both pages, which is what
	// separates a genuine continuous background from two ordinary pages
	// that merely share a thin margin of the same color at the binding.
	sharedFlatBg := leftSolid && rightSolid &&
		sd.marginColorsMatch(lMR, lMG, lMB, rMR, rMG, rMB) &&
		res.EdgeContinuity >= sd.FlatBgEdgeContinuityMin &&
		info >= sd.InfoFloor && info <= sd.FlatBgInfoMax &&
		sd.solidColorDepthFraction(lv, true, lMR, lMG, lMB) >= sd.FlatBgDepthMin &&
		sd.solidColorDepthFraction(rv, false, rMR, rMG, rMB) >= sd.FlatBgDepthMin

	// Step 5 — weighted score and penalties.
	//
	// EdgeContinuity and HistSimilarity match trivially when both seam
	// strips are uniform, so they are normally scaled by seam
	// informativeness. The shared-flat-background case is the deliberate
	// exception: there the uniform seam IS the spread, and a high
	// EdgeContinuity genuinely means the flat field continues across the
	// binding — so it is left unscaled.
	infoScale := info
	if sharedFlatBg {
		infoScale = 1.0
	}
	score := infoScale*(sd.WEdgeRMSE*res.EdgeContinuity+
		sd.WHistSim*res.HistSimilarity) +
		sd.WGradMatch*res.GradientMatch +
		sd.WEdgeAlign*res.EdgeAlignment

	// Strong seam continuity: the broad brightness pattern and the
	// structural edges both clearly continue across the binding, so a
	// real illustration is demonstrably flowing across the seam. When
	// this holds, an asymmetric-margin pre-filter hit is a false alarm
	// (a thick panel-border line misread as a page margin) and its
	// penalty is suppressed. See the StrongContinuity* fields.
	strongContinuity := res.SeamBrightnessCorrelation >= sd.StrongContinuityBright &&
		res.EdgeAlignment >= sd.StrongContinuityEdge

	var reasons []string
	if res.LeftBoxy || res.RightBoxy {
		score *= sd.BoxyPenalty
		reasons = append(reasons, "boxy/multi-panel page")
	}
	switch {
	case res.LeftHasMargin && res.RightHasMargin:
		// Both seam strips are solid. If they're the SAME color, that's
		// consistent with a shared illustration background flowing across
		// the seam (sparse art on a flat field) — not two independent page
		// margins meeting at the spine. Only different-color margins get
		// penalized; matching-color is left for the seam heuristics.
		if !sd.marginColorsMatch(lMR, lMG, lMB, rMR, rMG, rMB) {
			score *= sd.MarginMismatchPenalty
			reasons = append(reasons, "binding margins are different solid colors")
		}
	case res.LeftHasMargin != res.RightHasMargin:
		// Asymmetric: one side is a solid margin, the other has content
		// running up to the seam. A real spread can't have illustration
		// flowing into a flat strip — either both sides connect through
		// content, or both share a flat background. This pattern
		// indicates two independent pages, unless strong seam continuity
		// shows the "margin" was really a thick panel-border line with
		// content continuing across it.
		if !strongContinuity {
			score *= sd.MarginMismatchPenalty
			reasons = append(reasons, "only one binding edge is a solid-color margin")
		}
	}

	// Edge-only alignment penalty. A near-perfect Sobel/gradient match
	// with no matching broad-brightness continuity is the signature of
	// shared panel-frame lines at the binding edge, not a real spread.
	// See EdgeBrightDivergenceMax for the rationale.
	if res.EdgeAlignment-res.SeamBrightnessCorrelation > sd.EdgeBrightDivergenceMax {
		score *= sd.EdgeOnlyPenalty
		reasons = append(reasons, "edge alignment without tonal continuity")
	}

	// Row-wise alignment gate. EdgeContinuity and HistSimilarity are
	// global statistics that two unrelated manga pages can satisfy
	// trivially (both seams are mostly white). Only the three row-wise
	// correlation signals — GradientMatch, EdgeAlignment, and
	// SeamBrightnessCorrelation — measure actual row-by-row continuity,
	// each at a different frequency band:
	//   - SeamBrightnessCorrelation: low frequency (raw brightness)
	//   - GradientMatch:             mid frequency (derivative)
	//   - EdgeAlignment:             high frequency (Sobel)
	// A real spread shows up as a positive correlation in AT LEAST ONE
	// band. The gate fires only when all three sit near their chance
	// baseline; otherwise we trust the band(s) that did show alignment.
	// See CorrelationGateMin / BrightCorrGateMin for the per-band
	// threshold rationale. The shared-flat-background case is exempt: a
	// uniform field has no row structure to correlate, so chance-level
	// scores there are expected, not disqualifying.
	if !sharedFlatBg &&
		res.GradientMatch < sd.CorrelationGateMin &&
		res.EdgeAlignment < sd.CorrelationGateMin &&
		res.SeamBrightnessCorrelation < sd.BrightCorrGateMin {
		score *= sd.NoAlignmentPenalty
		reasons = append(reasons, "no row-wise alignment at the seam")
	}

	res.Confidence = clamp01(score)
	res.IsSpread = res.Confidence >= sd.SpreadThreshold

	// Blank-seam veto. When neither seam strip carries any variation —
	// both sides a uniform tone — there is nothing at the binding to
	// decide a spread either way. See InfoFloor for the rationale.
	if info < sd.InfoFloor {
		res.IsSpread = false
		reasons = append(reasons, "uniform seam carries no spread information")
	}

	switch {
	case len(reasons) > 0:
		res.Reason = fmt.Sprintf("%s; shift=%d; confidence %.2f",
			strings.Join(reasons, ", "), sh, res.Confidence)
	case res.IsSpread:
		res.Reason = fmt.Sprintf("spread likely (shift=%d, confidence %.2f)",
			sh, res.Confidence)
	default:
		res.Reason = fmt.Sprintf("spread unlikely (shift=%d, confidence %.2f)",
			sh, res.Confidence)
	}
	return res, nil
}

// ----------------------------- image view ------------------------------- //

// imgView wraps an image.Image for zero-based logical pixel access without
// copying pixel data. All reads go through the underlying image.Image.At,
// or through direct slice access for common concrete types via grayFunc/rgbFunc.
type imgView struct {
	img    image.Image
	ox, oy int // Bounds().Min offset
	w, h   int // Bounds().Dx(), Dy()
}

func newView(img image.Image) imgView {
	b := img.Bounds()
	return imgView{img, b.Min.X, b.Min.Y, b.Dx(), b.Dy()}
}

// rgb returns 8-bit color channels at logical zero-based (x, y).
// Use rgbFunc() in hot loops to avoid per-call interface dispatch.
func (v imgView) rgb(x, y int) (r, g, b float64) {
	r32, g32, b32, _ := v.img.At(v.ox+x, v.oy+y).RGBA()
	return float64(r32 >> 8), float64(g32 >> 8), float64(b32 >> 8)
}

// gray returns luminance at logical zero-based (x, y).
// Use grayFunc() in hot loops to avoid per-call interface dispatch.
func (v imgView) gray(x, y int) float64 {
	r, g, b := v.rgb(x, y)
	return 0.299*r + 0.587*g + 0.114*b
}

// grayFunc returns a closure for fast luminance access using direct slice
// reads for the common concrete image types decoded by image/jpeg and
// image/png. The type switch executes once; the returned function uses only
// array indexing in its hot path. Falls back to At() for unknown types.
func (v imgView) grayFunc() func(x, y int) float64 {
	ox, oy := v.ox, v.oy
	switch img := v.img.(type) {
	case *image.NRGBA:
		pix, stride := img.Pix, img.Stride
		return func(x, y int) float64 {
			i := (oy+y)*stride + (ox+x)*4
			return 0.299*float64(pix[i]) + 0.587*float64(pix[i+1]) + 0.114*float64(pix[i+2])
		}
	case *image.RGBA:
		pix, stride := img.Pix, img.Stride
		return func(x, y int) float64 {
			i := (oy+y)*stride + (ox+x)*4
			return 0.299*float64(pix[i]) + 0.587*float64(pix[i+1]) + 0.114*float64(pix[i+2])
		}
	case *image.YCbCr:
		// JPEG images: Y channel is luminance directly — no CbCr needed.
		yPix, yStride := img.Y, img.YStride
		return func(x, y int) float64 {
			return float64(yPix[(oy+y)*yStride+(ox+x)])
		}
	case *image.Gray:
		pix, stride := img.Pix, img.Stride
		return func(x, y int) float64 {
			return float64(pix[(oy+y)*stride+(ox+x)])
		}
	case *image.Gray16:
		pix, stride := img.Pix, img.Stride
		return func(x, y int) float64 {
			i := (oy+y)*stride + (ox+x)*2
			return float64(uint16(pix[i])<<8|uint16(pix[i+1])) / 256
		}
	default:
		return v.gray
	}
}

// rgbFunc returns a closure for fast RGB access using direct slice reads
// for the common concrete image types. Falls back to At() for unknown types.
func (v imgView) rgbFunc() func(x, y int) (r, g, b float64) {
	ox, oy := v.ox, v.oy
	switch img := v.img.(type) {
	case *image.NRGBA:
		pix, stride := img.Pix, img.Stride
		return func(x, y int) (r, g, b float64) {
			i := (oy+y)*stride + (ox+x)*4
			return float64(pix[i]), float64(pix[i+1]), float64(pix[i+2])
		}
	case *image.RGBA:
		pix, stride := img.Pix, img.Stride
		return func(x, y int) (r, g, b float64) {
			i := (oy+y)*stride + (ox+x)*4
			return float64(pix[i]), float64(pix[i+1]), float64(pix[i+2])
		}
	case *image.Gray:
		pix, stride := img.Pix, img.Stride
		return func(x, y int) (r, g, b float64) {
			lum := float64(pix[(oy+y)*stride+(ox+x)])
			return lum, lum, lum
		}
	default:
		return v.rgb
	}
}

// ---------------------------- pre-filters ------------------------------- //

// isBoxy reports whether the page looks like a multi-panel manga layout
// by detecting low-variance "gutter" bands that separate panels.
//
// A gutter is a band of consecutive rows OR columns whose brightness
// variance is below GutterVarianceMax. Bands touching the scan-axis
// edges (5% margin) are ignored — those are page margins or the binding
// edge, not internal panel boundaries.
//
// Tries three tests in order, returning true on the first hit:
//
//  1. ≥ BoxyHGutterMin horizontal gutters span the full page width
//     (stacked-panel layout).
//  2. ≥ BoxyVGutterMin vertical gutters span the full page height
//     (side-by-side panels).
//  3. Hierarchical band-aware test. Manga pages are typically divided
//     into horizontal bands first; each band is then split into panels
//     by short vertical gutters that DO NOT extend through the full page.
//     A full-page column scan misses those — the column reads as flat
//     in the top band but full of content in the bottom band, so its
//     overall variance is high. Fix: if there's at least one full-page
//     horizontal gutter, it cuts the page into inter-gutter bands;
//     re-scan each band for vertical gutters with the variance computed
//     over only that band's rows. If any band has ≥1 internal vertical
//     gutter, the page is a grid/mixed layout. Symmetric check for the
//     vertical-first split.
func (sd *SpreadDetector) isBoxy(v imgView) bool {
	hGutters := sd.findGutters(v, true, 0, v.w)
	vGutters := sd.findGutters(v, false, 0, v.h)

	if len(hGutters) >= sd.BoxyHGutterMin {
		return true
	}
	if len(vGutters) >= sd.BoxyVGutterMin {
		return true
	}

	// Hierarchical: horizontal-first. The most common manga case.
	if len(hGutters) >= 1 {
		for _, band := range bandsBetween(v.h, hGutters) {
			if band.extent() < sd.BoxyMinBandExtent {
				continue
			}
			if len(sd.findGutters(v, false, band.start, band.end)) >= 1 {
				return true
			}
		}
	}

	// Hierarchical: vertical-first (less common but symmetric).
	if len(vGutters) >= 1 {
		for _, band := range bandsBetween(v.w, vGutters) {
			if band.extent() < sd.BoxyMinBandExtent {
				continue
			}
			if len(sd.findGutters(v, true, band.start, band.end)) >= 1 {
				return true
			}
		}
	}

	return false
}

// interval is a half-open [start, end) range on one axis.
type interval struct{ start, end int }

func (iv interval) extent() int { return iv.end - iv.start }

// bandsBetween returns the inter-gutter intervals along an axis of
// length n. If gutters are at [r1, r2, ...], the bands are
//
//	[0, r1.start), [r1.end, r2.start), ..., [rN.end, n)
//
// (omitting any zero-extent intervals). Bands touching the page edge
// include any outer margin — that's fine, we just want the rows/cols
// where actual panel content lives.
func bandsBetween(n int, gutters []interval) []interval {
	out := make([]interval, 0, len(gutters)+1)
	cur := 0
	for _, gu := range gutters {
		if gu.start > cur {
			out = append(out, interval{cur, gu.start})
		}
		cur = gu.end
	}
	if cur < n {
		out = append(out, interval{cur, n})
	}
	return out
}

// findGutters returns the positions of gutters along one axis.
//
//   - If horizontal is true, scans rows; each gutter is a row range
//     [start, end). Variance per row is computed over the columns
//     x ∈ [otherStart, otherEnd).
//   - If horizontal is false, scans columns; each gutter is a column
//     range [start, end). Variance per column is computed over the rows
//     y ∈ [otherStart, otherEnd).
//
// Pass otherStart=0, otherEnd=full-axis-length to scan the full page.
// Pass a sub-range to look for in-band gutters (used for hierarchical
// layouts where a vertical gutter only exists within a horizontal band,
// or vice versa).
//
// A row/column counts as a gutter candidate if its variance is below
// GutterVarianceMax. Contiguous candidates are merged into runs; a run
// counts as a gutter if it is ≥ GutterMinThickness pixels thick AND
// ≤ GutterMaxThicknessFrac of the scan axis (thicker low-variance bands
// are flat artwork, not panel dividers) AND sits in the interior 90% of
// the scan axis (5% margin on each end is reserved for outer page
// margins / binding edges) AND is flanked on both sides by a
// substantial panel band (see filterFlankedGutters) AND passes the
// neighbor-contrast filter (the LESS-textured side must be at least
// GutterNeighborContrastMin × flatter than the band itself).
func (sd *SpreadDetector) findGutters(v imgView, horizontal bool, otherStart, otherEnd int) []interval {
	var n int
	if horizontal {
		n = v.h
	} else {
		n = v.w
	}
	m := otherEnd - otherStart
	if n == 0 || m <= 0 {
		return nil
	}

	// Per-slice variance (against the cross-section [otherStart, otherEnd)).
	// We keep the full array — not just an isGutter boolean — because the
	// neighbor-contrast filter below needs the variance of rows on either
	// side of each candidate band.
	//
	// grayAt is resolved once via a type switch; the hot loop only sees
	// direct slice indexing for the common image types (NRGBA, YCbCr, …).
	grayAt := v.grayFunc()
	variances := make([]float64, n)
	mf := float64(m)
	for a := 0; a < n; a++ {
		var sum, sumSq float64
		for b := otherStart; b < otherEnd; b++ {
			var val float64
			if horizontal {
				val = grayAt(b, a)
			} else {
				val = grayAt(a, b)
			}
			sum += val
			sumSq += val * val
		}
		mean := sum / mf
		variances[a] = sumSq/mf - mean*mean
	}

	edgeMargin := int(0.05 * float64(n))
	gutterMax := int(sd.GutterMaxThicknessFrac * float64(n))
	var out []interval
	for i := 0; i < n; {
		if variances[i] >= sd.GutterVarianceMax {
			i++
			continue
		}
		j := i
		for j < n && variances[j] < sd.GutterVarianceMax {
			j++
		}
		if j-i >= sd.GutterMinThickness && j-i <= gutterMax &&
			i > edgeMargin && j-1 < n-edgeMargin {
			if sd.isPanelGutter(variances, i, j, n) {
				out = append(out, interval{i, j})
			}
		}
		i = j
	}
	return sd.filterFlankedGutters(out, n)
}

// filterFlankedGutters keeps only those gutters that have a substantial
// panel band on BOTH sides — the band reaching to the neighbouring
// gutter (or the page edge). A gutter flanked by only a sliver is not
// separating two panels: it is texture clustered near an edge, or a
// chapter-title block, misread as a divider. n is the full length of
// the scan axis. See BoxyMinPanelExtentFrac.
func (sd *SpreadDetector) filterFlankedGutters(gutters []interval, n int) []interval {
	if len(gutters) == 0 {
		return gutters
	}
	minExtent := int(sd.BoxyMinPanelExtentFrac * float64(n))
	var out []interval
	for i, gu := range gutters {
		above := gu.start
		if i > 0 {
			above = gu.start - gutters[i-1].end
		}
		below := n - gu.end
		if i < len(gutters)-1 {
			below = gutters[i+1].start - gu.end
		}
		if above >= minExtent && below >= minExtent {
			out = append(out, gu)
		}
	}
	return out
}

// isPanelGutter rejects low-variance bands that aren't really panel
// gutters but rather flat regions of an illustration (uniform sky,
// uniform sea, halftone background). It checks that the band sits
// between two MARKEDLY more textured regions: the variance of the
// GutterNeighborWindow rows just above must be high, AND the variance
// of the same many rows just below must be high. Either side being
// flat means we're inside a uniform region, not between two panels.
//
// See the GutterNeighbor* fields for the threshold rationale.
func (sd *SpreadDetector) isPanelGutter(variances []float64, bandStart, bandEnd, n int) bool {
	aboveStart := bandStart - sd.GutterNeighborWindow
	if aboveStart < 0 {
		aboveStart = 0
	}
	belowEnd := bandEnd + sd.GutterNeighborWindow
	if belowEnd > n {
		belowEnd = n
	}
	if aboveStart >= bandStart || bandEnd >= belowEnd {
		// No room to measure a neighbor on at least one side.
		// This shouldn't happen given the edgeMargin already enforced
		// by the caller, but guard against degenerate inputs.
		return false
	}
	above := meanFloat(variances[aboveStart:bandStart])
	below := meanFloat(variances[bandEnd:belowEnd])
	band := meanFloat(variances[bandStart:bandEnd])
	if band < sd.GutterBandVarianceFloor {
		band = sd.GutterBandVarianceFloor
	}
	// The flatter neighbor must still be markedly more textured than
	// the band itself. min(), not max(): a real gutter requires content
	// on BOTH sides; content on only one side describes a content-to-flat
	// edge inside an illustration, not a panel divider.
	weakerNeighbor := above
	if below < weakerNeighbor {
		weakerNeighbor = below
	}
	return weakerNeighbor/band >= sd.GutterNeighborContrastMin
}

func meanFloat(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, v := range xs {
		sum += v
	}
	return sum / float64(len(xs))
}

// hasSolidColorMargin reports whether the requested binding edge of the
// page is dominated by a single solid color — of any hue — and returns
// the mean color of the seam strip (regardless of whether it qualified
// as solid). The mean is exposed because the caller needs to compare
// margin colors across the two pages: same color = shared background
// flowing across the seam, different colors = independent margins.
//
// Method: compute the mean color of the seam strip, then count pixels
// whose every channel sits within MarginChannelTol of that mean. If
// ≥ MarginFrac qualify, the strip is "solid".
func (sd *SpreadDetector) hasSolidColorMargin(v imgView, rightSide bool) (solid bool, mR, mG, mB float64) {
	sw := sd.seamWidth(v.w)
	total := sw * v.h
	if total == 0 {
		return false, 0, 0, 0
	}
	rgbAt := v.rgbFunc()
	var sR, sG, sB float64
	for y := 0; y < v.h; y++ {
		for d := 0; d < sw; d++ {
			x := d
			if rightSide {
				x = v.w - 1 - d
			}
			r, g, b := rgbAt(x, y)
			sR += r
			sG += g
			sB += b
		}
	}
	nf := float64(total)
	mR, mG, mB = sR/nf, sG/nf, sB/nf

	close := 0
	for y := 0; y < v.h; y++ {
		for d := 0; d < sw; d++ {
			x := d
			if rightSide {
				x = v.w - 1 - d
			}
			r, g, b := rgbAt(x, y)
			if math.Abs(r-mR) < sd.MarginChannelTol &&
				math.Abs(g-mG) < sd.MarginChannelTol &&
				math.Abs(b-mB) < sd.MarginChannelTol {
				close++
			}
		}
	}
	return float64(close)/nf >= sd.MarginFrac, mR, mG, mB
}

// marginColorsMatch reports whether two seam-strip mean colors are close
// enough to be considered the same hue. Used to distinguish a shared
// background flowing across the seam (skip the penalty) from two
// independent page margins (apply the penalty).
func (sd *SpreadDetector) marginColorsMatch(lR, lG, lB, rR, rG, rB float64) bool {
	return math.Abs(lR-rR) < sd.MarginMatchTol &&
		math.Abs(lG-rG) < sd.MarginMatchTol &&
		math.Abs(lB-rB) < sd.MarginMatchTol
}

// solidColorDepthFraction returns the fraction of pixels — within a wide
// strip extending FlatBgDepthFrac of the page width inward from the
// requested binding edge — that match the reference color (mR, mG, mB)
// within MarginChannelTol on every channel.
//
// It separates a thin page margin from a flat illustration background.
// A page margin is a narrow band of blank paper at the edge; only a few
// percent of the wide strip matches it, the rest being content. A flat
// background (a uniform night sky, a black void) keeps the match
// fraction high all the way across the strip.
func (sd *SpreadDetector) solidColorDepthFraction(v imgView, rightSide bool, mR, mG, mB float64) float64 {
	depth := int(sd.FlatBgDepthFrac * float64(v.w))
	if depth < 1 {
		depth = 1
	}
	if depth > v.w {
		depth = v.w
	}
	total := depth * v.h
	if total == 0 {
		return 0
	}
	rgbAt := v.rgbFunc()
	close := 0
	for y := 0; y < v.h; y++ {
		for d := 0; d < depth; d++ {
			x := d
			if rightSide {
				x = v.w - 1 - d
			}
			r, g, b := rgbAt(x, y)
			if math.Abs(r-mR) < sd.MarginChannelTol &&
				math.Abs(g-mG) < sd.MarginChannelTol &&
				math.Abs(b-mB) < sd.MarginChannelTol {
				close++
			}
		}
	}
	return float64(close) / float64(total)
}

// -------------------------- shift search -------------------------------- //

// findBestVerticalShift returns the row offset in [-maxShift, +maxShift]
// that maximises edge continuity. Searches by re-running the edge RMSE
// score at each candidate shift and picking the best.
func (sd *SpreadDetector) findBestVerticalShift(left, right imgView, maxShift int) int {
	bestShift := 0
	bestScore := -1.0
	for s := -maxShift; s <= maxShift; s++ {
		score := sd.edgeContinuityScore(left, right, s)
		if score > bestScore {
			bestScore = score
			bestShift = s
		}
	}
	return bestShift
}

// ---------------------------- seam heuristics --------------------------- //

// edgeContinuityScore: average K columns of color on each side of the seam,
// per row (with vertical shift), then RMSE in color space.
func (sd *SpreadDetector) edgeContinuityScore(left, right imgView, shift int) float64 {
	n := min(left.h, right.h)
	if n == 0 {
		return 0
	}
	kL := sd.edgeStrip(left.w)
	kR := sd.edgeStrip(right.w)
	lRGB := left.rgbFunc()
	rRGB := right.rgbFunc()
	var sumSq float64
	var count int
	for y := 0; y < n; y++ {
		ly := y * left.h / n
		ry := y*right.h/n + shift
		if ry < 0 || ry >= right.h {
			continue
		}

		var lR, lG, lB, rR, rG, rB float64
		for d := 0; d < kL; d++ {
			r, g, b := lRGB(left.w-1-d, ly)
			lR += r
			lG += g
			lB += b
		}
		for d := 0; d < kR; d++ {
			r, g, b := rRGB(d, ry)
			rR += r
			rG += g
			rB += b
		}
		lR /= float64(kL)
		lG /= float64(kL)
		lB /= float64(kL)
		rR /= float64(kR)
		rG /= float64(kR)
		rB /= float64(kR)

		dr := lR - rR
		dg := lG - rG
		db := lB - rB
		sumSq += (dr*dr + dg*dg + db*db) / 3
		count++
	}
	if count == 0 {
		return 0
	}
	rmse := math.Sqrt(sumSq / float64(count))
	return clamp01(1.0 - rmse/sd.EdgeRMSENormalizer)
}

// histogramSimilarityScore: 3D RGB histogram of the seam strip on each
// page, compared with the Bhattacharyya coefficient. Shift-independent.
func (sd *SpreadDetector) histogramSimilarityScore(left, right imgView) float64 {
	bpc := sd.ColorHistBinsPerChan
	total := bpc * bpc * bpc
	lh := make([]float64, total)
	rh := make([]float64, total)
	sd.accumStripColorHist(left, true, lh)
	sd.accumStripColorHist(right, false, rh)
	normalize(lh)
	normalize(rh)
	var bc float64
	for i := 0; i < total; i++ {
		bc += math.Sqrt(lh[i] * rh[i])
	}
	return clamp01(bc)
}

func (sd *SpreadDetector) accumStripColorHist(v imgView, rightSide bool, h []float64) {
	sw := sd.seamWidth(v.w)
	bpc := sd.ColorHistBinsPerChan
	scale := float64(bpc) / 256.0
	rgbAt := v.rgbFunc()
	for y := 0; y < v.h; y++ {
		for d := 0; d < sw; d++ {
			x := d
			if rightSide {
				x = v.w - 1 - d
			}
			r, g, b := rgbAt(x, y)
			br := int(r * scale)
			if br >= bpc {
				br = bpc - 1
			}
			bg := int(g * scale)
			if bg >= bpc {
				bg = bpc - 1
			}
			bb := int(b * scale)
			if bb >= bpc {
				bb = bpc - 1
			}
			h[br*bpc*bpc+bg*bpc+bb]++
		}
	}
}

// gradientMatchScore: Pearson correlation of the vertical derivatives of
// the per-row mean-brightness signals taken from each seam strip.
func (sd *SpreadDetector) gradientMatchScore(left, right imgView, shift int) float64 {
	lp, rp := sd.seamBrightnessProfiles(left, right, shift)
	if len(lp) < 2 {
		return 0
	}
	c := normalizedCorrelation(derivative(lp), derivative(rp))
	return clamp01((c + 1) / 2)
}

// seamBrightnessCorrelationScore: Pearson correlation of the raw per-row
// mean-brightness signals at the seam — the low-frequency counterpart to
// GradientMatch (no derivative). Catches real spreads whose broad
// brightness pattern continues across the seam even when the row-by-row
// derivatives don't correlate (e.g., when subtle registration drift or
// content jitter scrambles the high-frequency signal but leaves the
// overall illumination envelope aligned).
func (sd *SpreadDetector) seamBrightnessCorrelationScore(left, right imgView, shift int) float64 {
	lp, rp := sd.seamBrightnessProfiles(left, right, shift)
	if len(lp) < 2 {
		return 0
	}
	c := normalizedCorrelation(lp, rp)
	return clamp01((c + 1) / 2)
}

// seamBrightnessProfiles returns the per-row mean-brightness signals
// over the seam strip of each page, sampled proportionally to the
// shorter image and aligned by the given vertical shift.
func (sd *SpreadDetector) seamBrightnessProfiles(left, right imgView, shift int) (lp, rp []float64) {
	n := min(left.h, right.h)
	if n < 2 {
		return nil, nil
	}
	swL := sd.seamWidth(left.w)
	swR := sd.seamWidth(right.w)
	lGray := left.grayFunc()
	rGray := right.grayFunc()
	lp = make([]float64, 0, n)
	rp = make([]float64, 0, n)
	for y := 0; y < n; y++ {
		ly := y * left.h / n
		ry := y*right.h/n + shift
		if ry < 0 || ry >= right.h {
			continue
		}
		var sl, sr float64
		for d := 0; d < swL; d++ {
			sl += lGray(left.w-1-d, ly)
		}
		for d := 0; d < swR; d++ {
			sr += rGray(d, ry)
		}
		lp = append(lp, sl/float64(swL))
		rp = append(rp, sr/float64(swR))
	}
	return lp, rp
}

// horizontalEdgeAlignmentScore: Sobel edge magnitude at the boundary
// column of each page, per row; Pearson correlation of the two profiles.
// Isolates structural transitions (panel borders, outlines, hairlines)
// from broad tonal differences that RMSE conflates.
func horizontalEdgeAlignmentScore(left, right imgView, shift int) float64 {
	n := min(left.h, right.h)
	if n < 2 {
		return 0
	}
	lp := make([]float64, 0, n)
	rp := make([]float64, 0, n)
	for y := 0; y < n; y++ {
		ly := y * left.h / n
		ry := y*right.h/n + shift
		if ry < 0 || ry >= right.h {
			continue
		}
		lp = append(lp, sobelMag(left, left.w-1, ly))
		rp = append(rp, sobelMag(right, 0, ry))
	}
	if len(lp) < 2 {
		return 0
	}
	c := normalizedCorrelation(lp, rp)
	return clamp01((c + 1) / 2)
}

// sobelMag returns the Sobel gradient magnitude at (x, y), with border-
// replicate handling for out-of-image neighbours. This lets us compute a
// meaningful response even at the binding column where one side of the
// 3×3 kernel falls outside the page.
func sobelMag(v imgView, x, y int) float64 {
	grayAt := v.grayFunc()
	p := func(dx, dy int) float64 {
		xx := x + dx
		yy := y + dy
		if xx < 0 {
			xx = 0
		} else if xx >= v.w {
			xx = v.w - 1
		}
		if yy < 0 {
			yy = 0
		} else if yy >= v.h {
			yy = v.h - 1
		}
		return grayAt(xx, yy)
	}
	// Standard Sobel kernels:
	//   Gx = [-1 0 1; -2 0 2; -1 0 1]
	//   Gy = [-1 -2 -1; 0 0 0; 1 2 1]
	gx := -p(-1, -1) + p(1, -1) +
		-2*p(-1, 0) + 2*p(1, 0) +
		-p(-1, 1) + p(1, 1)
	gy := -p(-1, -1) - 2*p(0, -1) - p(1, -1) +
		p(-1, 1) + 2*p(0, 1) + p(1, 1)
	return math.Sqrt(gx*gx + gy*gy)
}

// ------------------------------ helpers --------------------------------- //

func (sd *SpreadDetector) seamWidth(w int) int {
	s := int(float64(w) * sd.SeamStripFrac)
	if s < sd.MinSeamStrip {
		s = sd.MinSeamStrip
	}
	if s > w {
		s = w
	}
	return s
}

func (sd *SpreadDetector) edgeStrip(w int) int {
	if sd.EdgeStripWidth > w {
		return w
	}
	return sd.EdgeStripWidth
}

// seamStripBrightnessVariance returns the variance of brightness over the
// entire seam strip (seamWidth columns × full height). Near 0 when the
// strip is a uniform color (margin, flat tone); rises quickly as soon as
// real content (lines, screentone, gradients) appears in the strip.
func (sd *SpreadDetector) seamStripBrightnessVariance(v imgView, rightSide bool) float64 {
	sw := sd.seamWidth(v.w)
	total := sw * v.h
	if total == 0 {
		return 0
	}
	grayAt := v.grayFunc()
	var sum, sumSq float64
	for y := 0; y < v.h; y++ {
		for d := 0; d < sw; d++ {
			x := d
			if rightSide {
				x = v.w - 1 - d
			}
			val := grayAt(x, y)
			sum += val
			sumSq += val * val
		}
	}
	nf := float64(total)
	mean := sum / nf
	return sumSq/nf - mean*mean
}

func derivative(p []float64) []float64 {
	d := make([]float64, len(p))
	for i := 1; i < len(p); i++ {
		d[i] = p[i] - p[i-1]
	}
	return d
}

func normalizedCorrelation(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var ma, mb float64
	for i := range a {
		ma += a[i]
		mb += b[i]
	}
	ma /= float64(len(a))
	mb /= float64(len(b))
	var num, da, db float64
	for i := range a {
		ax := a[i] - ma
		bx := b[i] - mb
		num += ax * bx
		da += ax * ax
		db += bx * bx
	}
	if da == 0 || db == 0 {
		return 0
	}
	return num / math.Sqrt(da*db)
}

func normalize(h []float64) {
	var sum float64
	for _, v := range h {
		sum += v
	}
	if sum == 0 {
		return
	}
	for i := range h {
		h[i] /= sum
	}
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
