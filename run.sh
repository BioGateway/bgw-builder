#!/bin/bash
if [ "$#" -lt 2 ]; then
  echo "Usage: ./run <versionNumber> <VOS directory>"
  exit 1
fi
sudo rm build.log
./build.sh $1 $2 20 >| build.log &
echo "Starting build in detached mode."
tail -f build.log