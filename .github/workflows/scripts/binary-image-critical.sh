#!/bin/bash

if [ ! -f "binary.txt" ];
then
    echo "Error: binary scan result file does not exist"
    exit 1
else
    while read -r line; do
      x=$(grep -o "CRITICAL: [0-9]*" | awk '{print $2}')
#      echo "printing the founded critical line"
#      echo "${x}"
    done < binary.txt
#    echo "the total sum is: $x"
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
