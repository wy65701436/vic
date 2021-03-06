// Copyright 2016 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"fmt"
	"regexp"

	log "github.com/Sirupsen/logrus"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/pkg/errors"
)

// https://kb.vmware.com/selfservice/microsites/search.do?language=en_US&cmd=displayKC&externalId=2046088
const unsuppCharsRegex = `%|&|\*|\$|#|@|!|\\|/|:|\?|"|<|>|;|'|\|`

// Same as unsuppCharsRegex but allows / and : for datastore paths
const unsuppCharsDatastoreRegex = `%|&|\*|\$|#|@|!|\\|\?|"|<|>|;|'|\|`

var reUnsupp = regexp.MustCompile(unsuppCharsRegex)
var reUnsuppDatastore = regexp.MustCompile(unsuppCharsDatastoreRegex)

func LogErrorIfAny(clic *cli.Context, err error) error {
	if err == nil {
		return nil
	}

	log.Errorf("--------------------")
	log.Errorf("%s %s failed: %s\n", clic.App.Name, clic.Command.Name, errors.ErrorStack(err))
	return cli.NewExitError("", 1)
}

// CheckUnsupportedChars returns an error if string contains special characters
func CheckUnsupportedChars(s string) error {
	return checkUnsupportedChars(s, reUnsupp)
}

// CheckUnsupportedCharsDatastore returns an error if a datastore string contains special characters
func CheckUnsupportedCharsDatastore(s string) error {
	return checkUnsupportedChars(s, reUnsuppDatastore)
}

func checkUnsupportedChars(s string, re *regexp.Regexp) error {
	st := []byte(s)
	var v []int
	if v = re.FindIndex(st); v == nil {
		return nil
	}
	return fmt.Errorf("unsupported character %q in %q", s[v[0]:v[1]], s)
}
