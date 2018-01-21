// Copyright © 2017 Delving B.V. <info@delving.eu>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hub3_test

import (
	"bitbucket.org/delving/rapid/hub3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Webresource", func() {

	Describe("hasher", func() {

		Context("When given a string", func() {

			It("should return a short hash", func() {
				hash := hub3.CreateHash("rapid rocks.")
				Expect(hash).To(Equal("a5b3be36c0f378a1"))
			})
		})

	})
})
