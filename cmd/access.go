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
	"regexp"
	"sort"
	"strings"

	"github.com/apppackio/apppack/auth"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/spf13/cobra"
)

func parameterValue(stack *cloudformation.Stack, key string) (*string, error) {
	for _, p := range stack.Parameters {
		if *p.ParameterKey == key {
			return p.ParameterValue, nil
		}
	}
	return nil, fmt.Errorf("cloudformation parameter %s not found", key)
}

func replaceParameter(stack *cloudformation.Stack, key string, value *string) error {
	for _, p := range stack.Parameters {
		if *p.ParameterKey == key {
			p.ParameterValue = value
			return nil
		}
	}
	return fmt.Errorf("cloudformation parameter %s not found", key)
}

func validateEmail(email string) bool {
	pattern := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	return pattern.MatchString(email)
}

func splitAndTrimCSV(csv *string) []string {
	var items []string
	for _, i := range strings.Split(*csv, ",") {
		items = append(items, strings.Trim(i, " "))
	}
	return items
}

func indexOf(arr []string, item string) int {
	for k, v := range arr {
		if item == v {
			return k
		}
	}
	return -1
}

func appOrPipelineStack(sess *session.Session, name string) (*cloudformation.Stack, error) {
	cfnSvc := cloudformation.New(sess)
	stackName := appStackName(AppName)
	stackOutput, err := cfnSvc.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: &stackName,
	})
	if err == nil {
		return stackOutput.Stacks[0], nil
	}
	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == "ValidationError" {
			stackName = pipelineStackName(AppName)
			stackOutput, err = cfnSvc.DescribeStacks(&cloudformation.DescribeStacksInput{
				StackName: &stackName,
			})
			if err == nil {
				return stackOutput.Stacks[0], nil
			}
		}
	}
	return nil, err
}

func adminSession() (*session.Session, error) {
	if UseAWSCredentials {
		if region != "" {
			return session.NewSession(&aws.Config{Region: &region})
		}
		sess, err := session.NewSession()
		if err != nil {
			return nil, err
		}
		if *sess.Config.Region == "" {
			return nil, fmt.Errorf("no region provided. Use the `--region` flag or set the AWS_REGION environment")
		}
		return sess, nil
	}
	sess, _, err := auth.AdminAWSSession(AccountIDorAlias)
	return sess, err
}

// accessCmd represents the access command
var accessCmd = &cobra.Command{
	Use:                   "access",
	Short:                 "list users with access to the app",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		startSpinner()
		var err error
		sess, err := adminSession()
		checkErr(err)
		stack, err := appOrPipelineStack(sess, AppName)
		checkErr(err)
		usersCSV, err := parameterValue(stack, "AllowedUsers")
		checkErr(err)
		users := splitAndTrimCSV(usersCSV)
		sort.Strings(users)
		Spinner.Stop()
		for _, u := range users {
			fmt.Println(u)
		}
	},
}

// accessAddCmd represents the access command
var accessAddCmd = &cobra.Command{
	Use:                   "add <email>",
	Short:                 "add access for a user to the app",
	Long:                  "*Requires admin permissions.*\nUpdates the application Cloudformation stack to add access for the user.",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		email := args[0]
		if !validateEmail(email) {
			checkErr(fmt.Errorf("%s does not appear to be a valid email address", email))
		}
		startSpinner()
		sess, err := adminSession()
		checkErr(err)
		stack, err := appOrPipelineStack(sess, AppName)
		checkErr(err)
		usersCSV, err := parameterValue(stack, "AllowedUsers")
		checkErr(err)
		usersCSV = aws.String(strings.Join([]string{*usersCSV, email}, ","))
		err = replaceParameter(stack, "AllowedUsers", usersCSV)
		checkErr(err)
		_, err = updateStackAndWait(sess, &cloudformation.UpdateStackInput{
			StackName:           stack.StackName,
			Parameters:          stack.Parameters,
			UsePreviousTemplate: aws.Bool(true),
			Capabilities:        []*string{aws.String("CAPABILITY_IAM")},
		})
		checkErr(err)
		Spinner.Stop()
		printSuccess(fmt.Sprintf("access added for %s on %s", email, AppName))
	},
}

// accessRemoveCmd represents the access command
var accessRemoveCmd = &cobra.Command{
	Use:                   "remove <email>",
	Short:                 "remove access for a user to the app",
	Long:                  "*Requires admin permissions.*\nUpdates the application Cloudformation stack to remove access for the user.",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		email := args[0]
		startSpinner()
		sess, err := adminSession()
		checkErr(err)
		stack, err := appOrPipelineStack(sess, AppName)
		checkErr(err)
		usersCSV, err := parameterValue(stack, "AllowedUsers")
		checkErr(err)
		userList := splitAndTrimCSV(usersCSV)
		idx := indexOf(userList, email)
		if idx < 0 {
			checkErr(fmt.Errorf("%s does not have access to %s", email, AppName))
		}
		newUsersCSV := strings.Join(append(userList[:idx], userList[idx+1:]...), ",")
		err = replaceParameter(stack, "AllowedUsers", &newUsersCSV)
		checkErr(err)
		_, err = updateStackAndWait(sess, &cloudformation.UpdateStackInput{
			StackName:           stack.StackName,
			Parameters:          stack.Parameters,
			UsePreviousTemplate: aws.Bool(true),
			Capabilities:        []*string{aws.String("CAPABILITY_IAM")},
		})
		checkErr(err)
		Spinner.Stop()
		printSuccess(fmt.Sprintf("access removed for %s on %s", email, AppName))
	},
}

func init() {
	rootCmd.AddCommand(accessCmd)

	accessCmd.PersistentFlags().StringVarP(&AppName, "app-name", "a", "", "app name (required)")
	accessCmd.MarkPersistentFlagRequired("app-name")
	accessCmd.PersistentFlags().StringVarP(&AccountIDorAlias, "account", "c", "", "AWS account ID or alias (not needed if you are only the administrator of one account)")
	accessCmd.PersistentFlags().BoolVar(&UseAWSCredentials, "aws-credentials", false, "use AWS credentials instead of AppPack.io federation")

	accessCmd.AddCommand(accessAddCmd)
	accessAddCmd.PersistentFlags().StringVar(&region, "region", "", "AWS region of app")
	accessCmd.AddCommand(accessRemoveCmd)
	accessRemoveCmd.PersistentFlags().StringVar(&region, "region", "", "AWS region of app")
}
