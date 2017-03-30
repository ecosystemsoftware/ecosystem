// Copyright 2017 Jonathan Pincas

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"

	"github.com/jpincas/ghost/auth"
	ghost "github.com/jpincas/ghost/tools"
	"github.com/jpincas/ghost/email"
	"github.com/jpincas/ghost/rest"
)

func main() {

	//Tell ghost which packages to activate
	ghost.ActivatePackages = activatePackages

	//Bootstrap the application
	if err := ghost.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func activatePackages() {
	//Standard packages
	rest.Activate()
	auth.Activate()
	email.Activate()
}
