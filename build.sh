#!/bin/bash

# Check if the number of arguments is less than 3
if [ "$#" -lt 3 ]; then
  echo "Error: Less than 3 arguments provided"
  exit 1
fi

# Cleanup existing directories
sudo rm -rf db/
sudo rm -rf target/
sudo rm -rf bgw-*/

# Start Docker services
docker compose up -d

# Prepare target directory
mkdir -p target/
# Start the copy operation in the background
cp -r "$2" target/vos && echo "Copy complete!" &
COPY_PID=$!  # Save the PID of the last background process

# Execute metadb-go in parallel to copying
vospath="$2"
vospath="${vospath%/}"
./metadb-go -path=$vospath/uploads -t=$3

# Wait for the copy operation to complete
wait $COPY_PID

# Shutdown Docker services
docker compose down

# Copy and set permissions
sudo cp -r db/ target/metadb
sudo chown -R $USER:docker target/
sudo chmod -R g+rw target/

# Replace placeholder and move the directory
sed 's/#version#/'$1'/' docker-template.yml > target/docker-compose.yml
mv target/ bgw-$1

echo "MetaDB build complete!"
