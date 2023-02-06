#!/bin/bash


if [ ! -f "scan.txt" ];
then
    echo "Error: scan result file does not exist"
    exit 1
else
   sed -n '2,4p' scan.txt > final.txt
   sed -n '7,9p' scan.txt > binary.txt
fi
