#!/bin/bash

sudo rm -r db/
docker compose up -d
./metadb-go -path $1 -t $2
docker compose down
sudo tar cvfz metadb.tgz db/
