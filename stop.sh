#!/bin/bash

# Huasteca Backend - Stop Script

echo "Stopping Huasteca Backend..."

# Stop Backend
pkill -f "huasteca-server-prod"
echo "Backend stopped."

# Uncomment below to also stop MongoDB
# pkill -x mongod
# echo "MongoDB stopped."
