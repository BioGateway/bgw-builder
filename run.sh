#!/bin/bash
if [ "$#" -lt 2 ]; then
  echo "Usage: ./run <versionNumber> <VOS directory>"
  exit 1
fi
rm build.log
sudo ./build.sh $1 $2 20 >| build.log &