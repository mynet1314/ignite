#! /bin/bash
docker build --build-arg VERSION=`git describe` -t mynet1314/nlan .
