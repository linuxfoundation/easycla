#!/bin/bash
curl -v -XPOST -H "Content-Type: application/json" -d@body.json http://localhost:5001/v4/notify-cla-managers
