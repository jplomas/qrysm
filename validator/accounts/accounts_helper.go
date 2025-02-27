package accounts

import (
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	dilithium2 "github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/qrysm/v4/cmd/validator/flags"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/io/prompt"
	"github.com/theQRL/qrysm/v4/validator/accounts/petnames"
	"github.com/theQRL/qrysm/v4/validator/accounts/userprompt"
	"github.com/urfave/cli/v2"
)

// selectAccounts Ask user to select accounts via an interactive user prompt.
func selectAccounts(selectionPrompt string, pubKeys [][dilithium2.CryptoPublicKeyBytes]byte) (filteredPubKeys []dilithium.PublicKey, err error) {
	pubKeyStrings := make([]string, len(pubKeys))
	for i, pk := range pubKeys {
		name := petnames.DeterministicName(pk[:], "-")
		pubKeyStrings[i] = fmt.Sprintf(
			"%d | %s | %#x", i, au.BrightGreen(name), au.BrightMagenta(bytesutil.Trunc(pk[:])),
		)
	}
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "\U0001F336 {{ .Name | cyan }}",
		Inactive: "  {{ .Name | cyan }}",
		Selected: "\U0001F336 {{ .Name | red | cyan }}",
		Details: `
--------- Account ----------
{{ "Name:" | faint }}	{{ .Name }}`,
	}
	var result string
	exit := "Done selecting"
	results := make([]int, 0)
	au := aurora.NewAurora(true)
	for result != exit {
		p := promptui.Select{
			Label:        selectionPrompt,
			HideSelected: true,
			Items:        append([]string{exit, allAccountsText}, pubKeyStrings...),
			Templates:    templates,
		}

		_, result, err = p.Run()
		if err != nil {
			return nil, err
		}
		if result == exit {
			fmt.Printf("%s\n", au.BrightRed("Done with selections").Bold())
			break
		}
		if result == allAccountsText {
			fmt.Printf("%s\n", au.BrightRed("[Selected all accounts]").Bold())
			for i := 0; i < len(pubKeys); i++ {
				results = append(results, i)
			}
			break
		}
		idx := strings.Index(result, " |")
		accountIndexStr := result[:idx]
		accountIndex, err := strconv.Atoi(accountIndexStr)
		if err != nil {
			return nil, err
		}
		results = append(results, accountIndex)
		fmt.Printf("%s %s\n", au.BrightRed("[Selected account]").Bold(), result)
	}

	// Deduplicate the results.
	seen := make(map[int]bool)
	for i := 0; i < len(results); i++ {
		if _, ok := seen[results[i]]; !ok {
			seen[results[i]] = true
		}
	}

	// Filter the public keys based on user input.
	filteredPubKeys = make([]dilithium.PublicKey, 0)
	for selectedIndex := range seen {
		pk, err := dilithium.PublicKeyFromBytes(pubKeys[selectedIndex][:])
		if err != nil {
			return nil, err
		}
		filteredPubKeys = append(filteredPubKeys, pk)
	}
	return filteredPubKeys, nil
}

// FilterPublicKeysFromUserInput collects the set of public keys from the
// command line or an interactive session.
func FilterPublicKeysFromUserInput(
	cliCtx *cli.Context,
	publicKeysFlag *cli.StringFlag,
	validatingPublicKeys [][dilithium2.CryptoPublicKeyBytes]byte,
	selectionPrompt string,
) ([]dilithium.PublicKey, error) {
	if cliCtx.IsSet(publicKeysFlag.Name) {
		pubKeyStrings := strings.Split(cliCtx.String(publicKeysFlag.Name), ",")
		if len(pubKeyStrings) == 0 {
			return nil, fmt.Errorf(
				"could not parse %s. It must be a string of comma-separated hex strings",
				publicKeysFlag.Name,
			)
		}
		return filterPublicKeys(pubKeyStrings)
	}
	return selectAccounts(selectionPrompt, validatingPublicKeys)
}

func filterPublicKeys(pubKeyStrings []string) ([]dilithium.PublicKey, error) {
	var filteredPubKeys []dilithium.PublicKey
	for _, str := range pubKeyStrings {
		pkString := str
		if strings.Contains(pkString, "0x") {
			pkString = pkString[2:]
		}
		pubKeyBytes, err := hex.DecodeString(pkString)
		if err != nil {
			return nil, errors.Wrapf(err, "could not decode string %s as hex", pkString)
		}
		dilithiumPublicKey, err := dilithium.PublicKeyFromBytes(pubKeyBytes)
		if err != nil {
			return nil, errors.Wrapf(err, "%#x is not a valid BLS public key", pubKeyBytes)
		}
		filteredPubKeys = append(filteredPubKeys, dilithiumPublicKey)
	}
	return filteredPubKeys, nil
}

// FilterExitAccountsFromUserInput selects which accounts to exit from the CLI.
func FilterExitAccountsFromUserInput(
	cliCtx *cli.Context,
	r io.Reader,
	validatingPublicKeys [][dilithium2.CryptoPublicKeyBytes]byte,
	forceExit bool,
) (rawPubKeys [][]byte, formattedPubKeys []string, err error) {
	if !cliCtx.IsSet(flags.ExitAllFlag.Name) {
		// Allow the user to interactively select the accounts to exit or optionally
		// provide them via cli flags as a string of comma-separated, hex strings.
		filteredPubKeys, err := FilterPublicKeysFromUserInput(
			cliCtx,
			flags.VoluntaryExitPublicKeysFlag,
			validatingPublicKeys,
			userprompt.SelectAccountsVoluntaryExitPromptText,
		)
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not filter public keys for voluntary exit")
		}
		rawPubKeys = make([][]byte, len(filteredPubKeys))
		formattedPubKeys = make([]string, len(filteredPubKeys))
		for i, pk := range filteredPubKeys {
			pubKeyBytes := pk.Marshal()
			rawPubKeys[i] = pubKeyBytes
			formattedPubKeys[i] = fmt.Sprintf("%#x", bytesutil.Trunc(pubKeyBytes))
		}
		allAccountStr := strings.Join(formattedPubKeys, ", ")
		if !cliCtx.IsSet(flags.VoluntaryExitPublicKeysFlag.Name) {
			if len(filteredPubKeys) == 1 {
				promptText := "Are you sure you want to perform a voluntary exit on 1 account? (%s) Y/N"
				resp, err := prompt.ValidatePrompt(
					r, fmt.Sprintf(promptText, au.BrightGreen(formattedPubKeys[0])), prompt.ValidateYesOrNo,
				)
				if err != nil {
					return nil, nil, err
				}
				if strings.EqualFold(resp, "n") {
					return nil, nil, nil
				}
			} else {
				promptText := "Are you sure you want to perform a voluntary exit on %d accounts? (%s) Y/N"
				if len(filteredPubKeys) == len(validatingPublicKeys) {
					promptText = fmt.Sprintf(
						"Are you sure you want to perform a voluntary exit on all accounts? Y/N (%s)",
						au.BrightGreen(allAccountStr))
				} else {
					promptText = fmt.Sprintf(promptText, len(filteredPubKeys), au.BrightGreen(allAccountStr))
				}
				resp, err := prompt.ValidatePrompt(r, promptText, prompt.ValidateYesOrNo)
				if err != nil {
					return nil, nil, err
				}
				if strings.EqualFold(resp, "n") {
					return nil, nil, nil
				}
			}
		}
	} else {
		rawPubKeys, formattedPubKeys = prepareAllKeys(validatingPublicKeys)
		fmt.Printf("About to perform a voluntary exit of %d accounts\n", len(rawPubKeys))
	}

	if forceExit {
		return rawPubKeys, formattedPubKeys, nil
	}

	promptHeader := au.Red("===============IMPORTANT===============")
	promptDescription := "Please navigate to the following website and make sure you understand the current implications " +
		"of a voluntary exit before making the final decision:"
	promptURL := au.Blue("https://docs.prylabs.network/docs/wallet/exiting-a-validator")
	promptQuestion := "If you still want to continue with the voluntary exit, please input a phrase found at the above URL"
	promptText := fmt.Sprintf("%s\n%s\n%s\n%s", promptHeader, promptDescription, promptURL, promptQuestion)
	resp, err := prompt.ValidatePrompt(r, promptText, func(input string) error {
		return prompt.ValidatePhrase(input, ExitPassphrase)
	})
	if err != nil {
		return nil, nil, err
	}
	if strings.EqualFold(resp, "n") {
		return nil, nil, nil
	}

	return rawPubKeys, formattedPubKeys, nil
}
