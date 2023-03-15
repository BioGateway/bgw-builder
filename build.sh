#!/bin/bash
sudo rm -r db/
sudo rm -r target/
sudo rm -r bgw-*/

docker compose up -d

mkdir -p target/
cp -r $2 target/vos &

./metadb-go -path=$2/uploads -t=$3
docker compose down
sudo cp -r db/ target/metadb
echo "MetaDB build complete!"

sed 's/#version#/'$1'/' docker-template.yml > target/docker-compose.yml

mv target/ bgw-$1
echo "Packaging..."
sudo tar cvfz biogateway-$1.tgz bgw-$1/
echo "Tarball complete!"