#!/bin/bash

echo "Directory name:"
read dir_name

if [ -d "$dir_name" ]; then
    echo "Directory exist"
else
    mkdir "$dir_name"
    echo "Directory created"
fi