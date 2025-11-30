package util

import (
	"errors"

	"github.com/AlecAivazis/survey/v2"
)

// SelectOne prompts the user to select one option from a list
func SelectOne(message string, options []string) (string, error) {
	if len(options) == 0 {
		return "", errors.New("no options provided")
	}

	var result string
	prompt := &survey.Select{
		Message: message,
		Options: options,
	}

	err := survey.AskOne(prompt, &result)
	if err != nil {
		return "", err
	}

	return result, nil
}

// SelectMultiple prompts the user to select multiple options from a list
func SelectMultiple(message string, options []string) ([]string, error) {
	if len(options) == 0 {
		return nil, errors.New("no options provided")
	}

	var result []string
	prompt := &survey.MultiSelect{
		Message: message,
		Options: options,
	}

	err := survey.AskOne(prompt, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Confirm prompts the user for a yes/no confirmation
func Confirm(message string, defaultVal bool) (bool, error) {
	var result bool
	prompt := &survey.Confirm{
		Message: message,
		Default: defaultVal,
	}

	err := survey.AskOne(prompt, &result)
	if err != nil {
		return false, err
	}

	return result, nil
}

// Input prompts the user for text input
func Input(message string, defaultVal string) (string, error) {
	var result string
	prompt := &survey.Input{
		Message: message,
		Default: defaultVal,
	}

	err := survey.AskOne(prompt, &result)
	if err != nil {
		return "", err
	}

	return result, nil
}
