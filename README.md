# BioGateway Deployment Builder
This tool is used to create the necessary database files and configuration for a deployment of BioGateway.
It will create a `docker compose` configuration for the following:
- A docker instance of Virtuoso, running on the appropriate version subdomain.
- A docker instance of the MetaDB MongoDB server.
- An instance of the MetaDB Server, connecting to the MetaDB MongoDB, and running on the appropriate version subdomain.

# Installation

Clone the repository with:
```
git clone git@github.com:BioGateway/bgw-builder.git

cd bgw-builder/
```

Run the following to fetch dependencies and build the GoLang program.
```
go get

go build
```

## Dependencies
This tool requires the following to be installed:
- Docker with Docker Compose
- `pigz` for parallel compression

The server where it is to be deployed also requires to be set up with `jwilder/nginx-proxy` and `nginxproxy/acme-companion` in order to map the subdomains correctly.

# Usage

### Building a BioGateway version
Run the following command in the current directory to build a new version of BioGateway

```bash
./build.sh <version number> <path to VOS folder> <number of threads>
```

_Example_
```bash
./build.sh 2309 ../vos/ 20
```
The example will build version 2309 of BioGateway from the files in the `../vos` directory, using 20 parallel threads.

### Output
The build script will produce a file named `biogateway-<version>.tgz` in the current directory.


# Deployment
The `.tgz` file can be copied and extracted where the docker deployments are supposed to be (currently `/data/docker/`),
and extracted.
Then, enter the extracted directory named `bgw-<version>`, and start the docker service with:
```bash
docker compose up -d
```

**Note: Before the first deployment of a new version, ensure that the subdomains for the version is appropriately set up in DNS at the domain provider.**
