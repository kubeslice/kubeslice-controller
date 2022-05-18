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
	"net"
	"strings"

	"go.uber.org/zap/zapcore"
)

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

func RemoveDuplicate(duplicate []string) (nonDup []string) {
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
