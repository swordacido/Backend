#!/bin/bash

# Huasteca Backend - Production Startup Script

echo "Starting Huasteca Backend..."

# Start MongoDB if not running
if ! pgrep -x "mongod" > /dev/null; then
    echo "Starting MongoDB..."
    mongod --dbpath /home/sword/Backend/mongodb-data \
           --logpath /home/sword/Backend/mongodb-data/mongod.log \
           --fork --port 27017 --nounixsocket
    sleep 2
fi

# Start Backend Server
cd /home/sword/Backend
echo "Starting backend server on port 8080..."
nohup ./huasteca-server-prod > server.log 2>&1 &

echo "Server started! Check server.log for details."
echo "Access the app at: http://localhost:8080"
