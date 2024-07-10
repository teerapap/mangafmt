#! /bin/sh
#
# build-releases.sh
# Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
#
# Distributed under terms of the MIT license.
#


rm -rf dist
mkdir -p dist

## Linux
echo "Build for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o dist/mangafmt-linux-amd64

## OSX
echo "Build for OSX (x86_64)..."
GOOS=darwin GOARCH=amd64 go build -o dist/mangafmt-osx-x86_64
echo "Build for OSX (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -o dist/mangafmt-osx-arm64

## Windows
echo "Build for Windows (i386)..."
GOOS=windows GOARCH=386 go build -o dist/mangafmt-win32.exe
echo "Build for Windows (x86_64)..."
GOOS=windows GOARCH=amd64 go build -o dist/mangafmt-win64.exe
echo "Build for Windows (arm64)..."
GOOS=windows GOARCH=arm64 go build -o dist/mangafmt-win64-arm.exe

echo "Done"
