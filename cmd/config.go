/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/lincolnloop/apppack/app"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

var AppName string

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage app configuration (environment variables/secrets)",
	Long:  `Configuration is stored in SSM Parameter Store and injected into the application containers at runtime.`,
}

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Read the value of a single config variable",
	Long:  `Read the value of a single config variable`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		startSpinner()
		a, err := app.Init(AppName)
		checkErr(err)
		svc := ssm.New(a.Session)
		resp, err := svc.GetParameter(&ssm.GetParameterInput{
			Name:           aws.String(fmt.Sprintf("/paaws/apps/%s/config/%s", AppName, args[0])),
			WithDecryption: aws.Bool(true),
		})
		Spinner.Stop()
		checkErr(err)
		fmt.Println(*resp.Parameter.Value)
	},
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all config variables and values",
	Long:  `List all config variables and values`,
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		w := new(tabwriter.Writer)
		// minwidth, tabwidth, padding, padchar, flags
		w.Init(os.Stdout, 8, 8, 0, '\t', 0)
		startSpinner()
		a, err := app.Init(AppName)
		checkErr(err)
		checkErr(err)
		svc := ssm.New(a.Session)
		resp, err := svc.GetParametersByPath(&ssm.GetParametersByPathInput{
			Path:           aws.String(fmt.Sprintf("/paaws/apps/%s/config/", AppName)),
			WithDecryption: aws.Bool(true),
		})
		Spinner.Stop()
		if err != nil {
			log.Fatalf("AWS API call failed: %v\n", err)
		}
		for _, value := range resp.Parameters {
			parts := strings.Split(*value.Name, "/")
			varname := parts[len(parts)-1]
			fmt.Fprintf(w, "%s\t%s\t\n", aurora.Green(fmt.Sprintf("%s:", varname)), *value.Value)
		}
		fmt.Println(aurora.Faint("==="), aurora.Bold(aurora.White(fmt.Sprintf("%s Config Vars", AppName))))
		w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.PersistentFlags().StringVarP(&AppName, "app-name", "a", "", "App name (required)")
	configCmd.MarkPersistentFlagRequired("app-name")

	configCmd.AddCommand(getCmd)
	configCmd.AddCommand(listCmd)
}
