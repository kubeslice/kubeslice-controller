#!/bin/bash

if [ ! -f "binary.txt" ];
then
    echo "Error: binary scan result file does not exist"
    exit 1
else
    x=$(grep -o "CRITICAL: [0-9]*" binary.txt | awk '{print $2}')
fi

sum=0
for i in $x; do
        sum=$((sum+i))
done
echo " binary image critical value is  $sum"

if [ $sum -gt 0 ]
then
   echo "CRITICAL vulnerabilities found"
   exit 1
else
   echo "no CRITICAL vulnerabilities found"
   exit 0
fi
