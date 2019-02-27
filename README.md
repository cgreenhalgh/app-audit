# Audit App

## Testing outside of databox

This app won't work outside databox

## Running on databox

To get running on the databox, you will first need to create a docker container to run your code.  To do this, you can either build the container on your databox, so pull your code, then in the src directory type:

Change or override DEFAULT_REG and VERSION in the Makefile if you want, but be sure databox-manifest.json matches.

For x86 platforms:
```
make build-amd64 
```

For Arm v8 platforms:
```
make build-arm64v8 
```

This will build and tag a docker image for use with databox. If databox is running on a machine other than the one you used to build the image then you will need to push it to docker hub under your own account. Change DEFAULT_REG to your docker hub registry, push it then pull it on the the target box the retag to databoxsystems.

Finally, you'll need to upload your manifest file to tell databox about the new app.  Log in to the databox and navigate to My Apps, then click on the "app store" app.  At the bottom of the page, use the form to upload your manifest.  Once uploaded, you can navigate to "App Store" and you should see go-test-app ready to install.

## Development

Build with `Dockerfile.dev` (amd64) and tag (change :
```
docker build -t cgreenhalgh/app-audit-amd64:0.5.2 -f Dockerfile.dev .
```
Upload manifest, install app.

Copy new files in with docker cp

Build and run with 
```
docker ps | grep app-audit
docker exec -it CONTAINER /bin/sh
GGO_ENABLED=0 GOOS=linux go build -a -ldflags '-s -w' -o app src/*.go
./app
```

## Status / to do

Status:
- observes CM API - see install request (w manifest), which includes other stores' datasources that it uses and its resource requirements, e.g. "store". Also includes "provides" manifest hint that is missing in SLA
- does initial get of existing SLAs from CM slaStore (again, includes datasources used but not provided)

To do:
- identify generated datasources, from associated store hypercat?!
- audit generated datasources
- audit used datasources
- audit UI ??
- audit network ??
- audit export ???
