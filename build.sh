#!/bin/bash
if [ "$#" -lt 3 ]; then
  echo "Error: Less than 3 arguments provided"
  exit 1
fi

sudo rm -r db/
sudo rm -r target/
sudo rm -r bgw-*/

docker compose up -d

mkdir -p target/
cp -r $2 target/vos &
vospath="$2"
vospath="${vospath%/}"

./metadb-go -path=$vospath/uploads -t=$3
docker compose down
sudo cp -r db/ target/metadb
sudo chown -R $USER:docker target/
sudo chmod -R g+rw target/

sed 's/#version#/'$1'/' docker-template.yml > target/docker-compose.yml

mv target/ bgw-$1
echo "MetaDB build complete!"

# echo "Packaging..."

# tar cvf - bgw-$1/ | pigz > biogateway-$1.tgz
# echo "Tarball complete!"