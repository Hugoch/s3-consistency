#!/bin/bash

# Cross-compile script based on https://gist.github.com/eduncan911/68775dba9d3c028181e4 & https://github.com/dvassallo/s3-benchmark

PLATFORMS="darwin/amd64" # amd64 only as of go1.5
PLATFORMS="$PLATFORMS windows/amd64" # arm compilation not available for Windows
PLATFORMS="$PLATFORMS linux/amd64"
PLATFORMS_ARM="linux"

type setopt >/dev/null 2>&1
SCRIPT_NAME=`basename "$0"`
FAILURES=""
SOURCE_FILE=`echo $@ | sed 's/\.go//'`
CURRENT_DIRECTORY=${PWD##*/}
OUTPUT=${SOURCE_FILE:-$CURRENT_DIRECTORY} # if no src file given, use current dir name
for PLATFORM in $PLATFORMS; do
  GOOS=${PLATFORM%/*}
  GOARCH=${PLATFORM#*/}
  BIN_FILENAME="${OUTPUT}"
if [[ "${GOOS}" == "windows" ]]; then BIN_FILENAME="${BIN_FILENAME}.exe"; fi
  mkdir -p "build/${GOOS}-${GOARCH}/"
  CMD="GOOS=${GOOS} GOARCH=${GOARCH} go build -o build/${GOOS}-${GOARCH}/${BIN_FILENAME} $@"
echo "${CMD}"
eval $CMD || FAILURES="${FAILURES} ${PLATFORM}"
done
# ARM builds
if [[ $PLATFORMS_ARM == *"linux"* ]]; then
  mkdir -p "build/${GOOS}-arm64/"
  CMD="GOOS=linux GOARCH=arm64 go build -o build/${GOOS}-arm64/${OUTPUT} $@"
echo "${CMD}"
eval $CMD || FAILURES="${FAILURES} ${PLATFORM}"
fi

# eval errors
if [[ "${FAILURES}" != "" ]]; then
echo ""
echo "${SCRIPT_NAME} failed on: ${FAILURES}"
exit 1
fi