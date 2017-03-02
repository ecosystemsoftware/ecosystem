// Copyright © 2017 NAME HERE <EMAIL ADDRESS>
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

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/ecosystemsoftware/ecosystem/core"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var isInstallDemoData, isReinstall bool

func init() {
	RootCmd.AddCommand(installCmd)
	RootCmd.AddCommand(unInstallCmd)
	installCmd.Flags().BoolVar(&isInstallDemoData, "demodata", false, "Install bundle demo data if available")
	installCmd.Flags().BoolVarP(&isReinstall, "reinstall", "r", false, "Uninstall bundle before installing")
}

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install [bundle]",
	Short: "Install an EcoSystem bundle",
	Long: `Installs an EcoSystem bundle from the named folder.
	Note: does not download anything, so the bundle folder must
	exist and contain everything.  Previous to installing, either clone
	or download the bundle into the 'bundles' directory`,
	RunE: installBundle,
}

// installCmd represents the install command
var unInstallCmd = &cobra.Command{
	Use:   "uninstall [bundle]",
	Short: "Removes an EcoSystem bundle",
	Long: `Removes an EcoSystem bundle by deleting the DB schema, deleting template
	files and images,`,
	RunE: unInstallBundle,
}

//uninstallBundle is the removal function for a bundle
func unInstallBundle(cmd *cobra.Command, args []string) error {

	//Check for bundle name
	if len(args) < 1 {
		return errors.New("a bundle name must be provided")
	}

	//Ask for confirmation
	c := core.AskForConfirmation("This will delete the bundle, causing loss of all data in the schema created by the bundle.  Are you sure you want to do this?")

	if c {
		//Establish a temporary connection as the super user
		db := core.SuperUserDBConfig.ReturnDBConnection("")
		defer db.Close()

		//Delete the web categories installed by this bundle
		db.Exec(fmt.Sprintf(core.SQLToDeleteBundleCategories, args[0]))

		//Drop the schema
		//If it doesn't exist, it won't be dropped - no big deal
		db.Exec(fmt.Sprintf(core.SQLToDropSchema, args[0]))

		//Attempt to updated the bundles installed list
		newBundlesInstalled, err := core.Bundles(viper.GetStringSlice("bundlesInstalled")).UnInstallBundle(args[0])

		//If there is any error, return it
		if err != nil {
			log.Println("Error updating bundles installed list: ", err.Error())
		}

		//Otherwise set the viper configuration to the new bundles list and overwrite the config.json
		viper.Set("bundlesInstalled", newBundlesInstalled)
		var config core.Config
		viper.Unmarshal(&config)
		configJSON, _ := json.MarshalIndent(config, "", "\t")
		err = ioutil.WriteFile("config.json", configJSON, 0644)
		if err != nil {
			log.Println("Error updating config.json: ", err.Error())
		}

		log.Println("config.json updated")

		log.Println("Uninstallation of bundle", args[0], "completed")
	}

	return nil

}

//installBundle is the entire installation procedure for an EcoSystem Bundle
func installBundle(cmd *cobra.Command, args []string) error {

	//Check for bundle name
	if len(args) < 1 {
		return errors.New("a bundle name must be provided")
	}

	//Check that bundle exists
	basePath := "./bundles/" + args[0]
	exists, _ := afero.IsDir(core.AppFs, basePath)
	if !exists {
		//Exit if doesn't exist
		log.Fatal("Bundle ", args[0], " not found.  Please download or clone.")
	}

	//Uninstall first if requested
	if isReinstall {
		log.Println("Proceeding to uninstall bundle ", args[0], " before reinstallation")
		unInstallBundle(cmd, args)
	}

	//Establish a temporary connection as the super user
	db := core.SuperUserDBConfig.ReturnDBConnection("")
	defer db.Close()

	//Check for the presence of install.sql and attempt to read it
	sqlBytes, err := afero.ReadFile(core.AppFs, basePath+"/install.sql")
	if err != nil {
		log.Println("install.sql not present for this bundle, or could not be read: ", err.Error())
	} else {
		sqlString := string(sqlBytes)

		//Install the DB setup and logic
		//Attempt to create a schema matching the bundle's name,
		_, err := db.Exec(fmt.Sprintf(core.SQLToCreateSchema, args[0]))

		if err != nil {

			//if the schema exists, the bundle is already installed
			//don't go any further with db part of the bundle
			log.Println("Failed to create DB schema: ", err.Error())

		} else {

			//Set admin privileges for everything in this schema going forwards
			_, err = db.Exec(fmt.Sprintf(core.SQLToGrantBundleAdminPermissions, args[0], args[0]))

			//Set the search path to the bundle schema so that all SQL commands take
			//place within the schema
			_, err = db.Exec(fmt.Sprintf(core.SQLToSetSearchPathForBundle, args[0]))
			if err != nil {

				//If there is any problem with the search path, give up the db part of the bundle
				log.Println("search_path failed to set, aborting sql installation and cleaning up", err.Error())
				db.Exec(fmt.Sprintf(core.SQLToDropSchema, args[0]))

			} else {

				//Run the SQL
				_, err = db.Exec(sqlString)
				if err != nil {
					log.Println("Problem with install.sql, aborting sql installation and cleaning up", err.Error())
					db.Exec(fmt.Sprintf(core.SQLToDropSchema, args[0]))
				} else {

					//If all is good so far and the user has specified it, install the demo data (if it exists)
					if isInstallDemoData {

						//Check for the presence of demodata.sql and attempt to read it
						sqlBytes, err := afero.ReadFile(core.AppFs, basePath+"/demodata.sql")
						if err != nil {
							//If there is no demodata.sql
							log.Println("demodata.sql not present for this bundle, or could not be read: ", err.Error())
						} else {
							sqlString := string(sqlBytes)
							_, err = db.Exec(sqlString)
							if err != nil {
								//If there is an error with demodata.sql
								log.Println("Error installing demo data: ", err.Error())
							}
						}
					}
				}
			}
		}
	} //End sql installation

	//Attempt to updated the bundles installed list
	newBundlesInstalled, err := core.Bundles(viper.GetStringSlice("bundlesInstalled")).InstallBundle(args[0])

	//If there is any error, return it
	if err != nil {
		log.Println("Error updating bundles installed list: ", err.Error())
	}

	//Otherwise set the viper configuration to the new bundles list and overwrite the config.json
	viper.Set("bundlesInstalled", newBundlesInstalled)
	var config core.Config
	viper.Unmarshal(&config)
	configJSON, _ := json.MarshalIndent(config, "", "\t")
	err = ioutil.WriteFile("config.json", configJSON, 0644)
	if err != nil {
		log.Println("Error updating config.json: ", err.Error())
	}

	log.Println("config.json updated")

	//Bundle installation complete
	log.Println("Installation of bundle", args[0], "completed")
	return nil

}
