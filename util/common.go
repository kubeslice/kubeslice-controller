/*
 * 	Copyright (c) 2022 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
 *
 * 	Licensed under the Apache License, Version 2.0 (the "License");
 * 	you may not use this file except in compliance with the License.
 * 	You may obtain a copy of the License at
 *
 * 	http://www.apache.org/licenses/LICENSE-2.0
 *
 * 	Unless required by applicable law or agreed to in writing, software
 * 	distributed under the License is distributed on an "AS IS" BASIS,
 * 	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * 	See the License for the specific language governing permissions and
 * 	limitations under the License.
 */

package util

import (
	"context"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
)

type WorkerSliceGatewayNetworkAddresses struct {
	ServerNetwork    string
	ClientNetwork    string
	ServerSubnet     string
	ClientSubnet     string
	ServerVpnNetwork string
	ServerVpnAddress string
	ClientVpnAddress string
}

// AppendHyphenToString is a function add hyphen at the end of string
func AppendHyphenToString(stringToAppend string) string {
	if strings.HasSuffix(stringToAppend, "-") {
		return stringToAppend
	} else {
		return stringToAppend + "-"
	}
}

// AppendHyphenAndPercentageSToString is a function to add hyphen and % at the end of string
func AppendHyphenAndPercentageSToString(stringToAppend string) string {
	stringToAppend = AppendHyphenToString(stringToAppend)
	stringToAppend = stringToAppend + "%s"
	return stringToAppend
}

// IsInSlice function for check if element is in array slice
func IsInSlice(slice []string, element string) bool {
	for _, value := range slice {
		if value == element {
			return true
		}
	}
	return false
}

// GetZapLogLevel is function to add the layer of debug, error, info in logging
func GetZapLogLevel(userLogLevel string) zapcore.Level {
	switch userLogLevel {
	case "debug":
		return zapcore.DebugLevel
	case "error":
		return zapcore.ErrorLevel
	case "info":
		return zapcore.InfoLevel
	default:
		return zapcore.InfoLevel
	}
}

// IsPrivateSubnet function for check if prefix matches any item in a slice
func IsPrivateSubnet(subnet string) bool {
	ipv4Addr, _, err := net.ParseCIDR(subnet)
	return err == nil && ipv4Addr.IsPrivate()
}

// HasPrefix is function to check if the string has given prefix
func HasPrefix(subnet string, prefix string) bool {
	return subnet[len(subnet)-2:] == prefix
}

// HasLastTwoOctetsZero is a function to check if the subnet address's last octet is 0
func HasLastTwoOctetsZero(subnet string) bool {
	ipv4Addr, _, err := net.ParseCIDR(subnet)
	if err == nil {
		s := strings.Split(ipv4Addr.String(), ".")
		if s[2] == "0" && s[3] == "0" {
			return true
		}
	}
	return false
}

// OverlapIP function to check whether two IP addresses overlap each other
func OverlapIP(ip1, ip2 string) bool {
	_, n1, err1 := net.ParseCIDR(ip1)
	_, n2, err2 := net.ParseCIDR(ip2)
	if err1 != nil || err2 != nil {
		return false
	}
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}

// CheckDuplicateInArray check duplicate data in array
func CheckDuplicateInArray(data []string) (bool, []string) {
	set := make(map[string]bool)
	var dup []string
	for _, v := range data {
		if set[v] {
			dup = append(dup, v)
		}
		set[v] = true
	}
	if len(dup) > 0 {
		return true, dup
	}
	return false, dup
}

func RemoveDuplicatesFromArray(duplicate []string) (nonDup []string) {
	mp := make(map[string]bool)
	for i := 0; i < len(duplicate); i++ {
		if _, ok := mp[duplicate[i]]; ok {
			continue
		} else {
			mp[duplicate[i]] = true
			nonDup = append(nonDup, duplicate[i])
		}
	}
	return nonDup
}

func ArrayToString(arr []string) string {
	var str string
	str = strings.Join(arr, ",")
	return str
}

// FindCIDRByMaxClusters is a function to find the CIDR by max clusters
func FindCIDRByMaxClusters(maxCluster int) string {
	var cidr string
	baseCidr := 17
	for i := 7; i >= 0; i-- {
		if float64(maxCluster) > math.Pow(2, float64(i)) {
			value := i + baseCidr
			cidr = fmt.Sprintf("/%d", value)
			break
		}
	}
	return cidr
}

// intPow compute a**b using binary powering algorithm
func intPow(a, b int) int {
	p := 1
	for b > 0 {
		if b&1 != 0 {
			p *= a
		}
		b >>= 1
		a *= a
	}
	return p
}

// subnetOctetDiff calculates the controlling octet of the subnet along with the difference factor of the octet from cidr
// for eg: a cidr of /20 means 3rd octet controls the subnet and has a difference of 16 (3, 16)
// returns -1, -1 for invalid cidr
func subnetOctetDiff(cidr int) (int, int) {
	if cidr <= 24 && cidr > 0 {
		cidrDiv := 32 / cidr
		return 4 - cidrDiv, intPow(2, 32-cidr) / (256 * cidrDiv)
	} else if cidr > 24 && cidr <= 32 {
		return 4, intPow(2, 32-cidr)
	}
	return -1, -1
}

func GetClusterPrefixPool(sliceSubnet string, ipamInt int, subnetCidr string) string {
	cidrInt := strings.Replace(subnetCidr, "/", "", -1)
	cidr, _ := strconv.Atoi(cidrInt)
	octetList := strings.Split(sliceSubnet, ".")
	controlOctet, controlOctetDiff := subnetOctetDiff(cidr)
	ipamInt *= controlOctetDiff
	ipamOctet := strconv.Itoa(ipamInt)
	octetList[controlOctet-1] = ipamOctet
	for i := controlOctet; i <= 3; i++ {
		octetList[i] = "0"
	}
	octetList[3] += fmt.Sprintf("/%d", cidr)
	return strings.Join(octetList, ".")
}

// Retry tries to execute the funtion, If failed reattempts till backoffLimit
func Retry(ctx context.Context, backoffLimit int, sleep time.Duration, f func() error) (err error) {
	logger := CtxLogger(ctx)
	start := time.Now()
	for i := 0; i < backoffLimit; i++ {
		if i > 0 {
			logger.Infof("%s Waiting %d seconds before retrying as an error occured: %s", Wait, int(sleep.Seconds()), err.Error())
			time.Sleep(sleep)
			sleep *= 2
		}
		err = f()
		if err == nil {
			return nil
		}
	}
	elapsed := time.Since(start)
	return fmt.Errorf("retry failed after %d attempts (took %d seconds), last error: %s", backoffLimit, int(elapsed.Seconds()), err)
}

// Set Difference: A - B
func DifferenceOfArray(a, b []string) (diff []string) {
	m := make(map[string]bool)

	for _, item := range b {
		m[item] = true
	}

	for _, item := range a {
		if _, ok := m[item]; !ok {
			diff = append(diff, item)
		}
	}
	return diff
}

func RemoveElementFromArray(slice []string, element string) (arr []string) {
	for i, v := range slice {
		if v == element {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

var toFilter = []string{"kubeslice-", "kubernetes.io"}

func partialContains(slice []string, str string) bool {
	for _, s := range slice {
		if strings.Contains(str, s) {
			return true
		}
	}
	return false
}

func FilterLabelsAndAnnotations(data map[string]string) map[string]string {
	filtered := make(map[string]string)
	for key, value := range data {
		// Skip if `key` contains any substring in `toFilter`
		if !partialContains(toFilter, key) {
			filtered[key] = value
		}
	}
	return filtered
}
