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
	"errors"
	"regexp"
	"strconv"
)

// IsDNSCompliant is a function to check if the given string/name is DNS compliant
func IsDNSCompliant(name string) bool {
	pattern := "^[a-zA-Z0-9][a-zA-Z0-9\\-\\.]{0,62}[a-zA-Z0-9]$"
	compiledRegex, _ := regexp.Compile(pattern)
	match := compiledRegex.Match([]byte(name))
	return match
}

func ValidateCoOrdinates(latitude string, longitude string) error {
	coord1, err1 := strconv.ParseFloat(latitude, 64)
	coord2, err2 := strconv.ParseFloat(longitude, 64)
	if err1 != nil || err2 != nil || coord1 < -90 || coord1 > 90 || coord2 < -180 || coord2 > 180 {
		err := errors.New("Latitude and longitude are not valid")
		return err
	}
	return nil
}
