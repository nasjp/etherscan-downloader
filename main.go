package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// set here
const target = "v1punks"

//////////////////////////
//////////////////////////
//////////////////////////
//////////////////////////

const contractDir = "contracts"

const (
	ethereum chain = 1
	polygon  chain = 137
)

var blockExploers = map[chain]blockExplorer{
	ethereum: {endpoint: "https://api.etherscan.io/", apiKey: os.Getenv("ETHERSCAN_APIKEY")},
	polygon:  {endpoint: "https://api.polygonscan.com/", apiKey: os.Getenv("POLYGONSCAN_APIKEY")},
}

var targetAddresses = map[string]evmAddress{
	"cryp_toadz":       {Chain: ethereum, Address: "0x1cb1a5e65610aeff2551a50f76a87a7d3fb649c6"},
	"generative_masks": {Chain: ethereum, Address: "0x80416304142fa37929f8a4eee83ee7d2dac12d7c"},
	"gal_verse":        {Chain: ethereum, Address: "0x582048C4077a34E7c3799962F1F8C5342a3F4b12"},
	"beefy":            {Chain: ethereum, Address: "0x18a20abeba0086ac0c564B2bA3a7BaF18568667D"},
	"beefy_strategy":   {Chain: ethereum, Address: "0xBaBaC5560Aa4CA3C5290DfcC5C159EdC0a51c316"},
	"beefy_chef":       {Chain: ethereum, Address: "0x0769fd68dFb93167989C6f7254cd0D766Fb2841F"},
	"convex_booster":   {Chain: ethereum, Address: "0xF403C135812408BFbE8713b5A23a04b3D48AAE31"},
	"pooltogether":     {Chain: ethereum, Address: "0xbc82221e131c082336cf698f0ca3ebd18afd4ce7"},
	"moonbirds":        {Chain: ethereum, Address: "0x23581767a106ae21c074b2276d25e5c3e136a68b"},
	"v1punks":          {Chain: ethereum, Address: "0x282bdd42f4eb70e7a9d9f40c8fea0825b7f68c5d"},
	"space_doodles":    {Chain: ethereum, Address: "0x620b70123fb810f6c653da7644b5dd0b6312e4d8"},
}

type blockExplorer struct {
	endpoint string
	apiKey   string
}

type chain uint

type evmAddress struct {
	Chain   chain
	Address string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	targetAddress := targetAddresses[target]
	explorer := blockExploers[targetAddress.Chain]

	rawCodes, err := getRawContractCode(explorer.endpoint, targetAddress.Address, explorer.apiKey)
	if err != nil {
		return err
	}

	sourceCodes, err := parseContractCode(rawCodes)
	if err != nil {
		return err
	}

	for _, sourceCode := range sourceCodes {
		for path, source := range sourceCode.Sources {
			if err := os.MkdirAll(targetDir(target, path), os.ModePerm); err != nil {
				return err
			}

			f, err := os.Create(targetPath(target, path))
			if err != nil {
				return err
			}
			defer f.Close()

			f.Write([]byte(source.Content))
		}
	}

	return nil
}

func getContractURL(endpoint string, address string, apikey string) string {
	const url = "%s/api?module=contract&action=getsourcecode&address=%s&apikey=%s"
	return fmt.Sprintf(url, endpoint, address, apikey)
}

func targetDir(dir string, path string) string {
	return filepath.Dir(targetPath(dir, path))
}

func targetPath(dir string, path string) string {
	return filepath.Join(contractDir, dir, path)
}

func getRawContractCode(endpoint, address string, apiKey string) ([]*RawCode, error) {
	url := getContractURL(endpoint, address, apiKey)
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return nil, err
	}

	contractCodeResponse := &Response{}

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.NewDecoder(bytes.NewBuffer(bs)).Decode(contractCodeResponse); err != nil {
		return []*RawCode{{SourceCode: string(bs), IsOneSource: true}}, nil
	}

	if contractCodeResponse.Status != "1" {
		return nil, fmt.Errorf("bad status: %s, message: %s", contractCodeResponse.Status, contractCodeResponse.Status)
	}

	return contractCodeResponse.Codes, nil
}

func parseContractCode(rawCodes []*RawCode) ([]*SourceCode, error) {
	sourceCodes := make([]*SourceCode, 0, len(rawCodes))
	if len(rawCodes) == 1 && rawCodes[0].IsOneSource {
		return []*SourceCode{{
			Sources: Sources{"main.sol": &Contract{Content: rawCodes[0].SourceCode}},
		}}, nil
	}

	for _, rawCode := range rawCodes {
		sourceCode := &SourceCode{}
		if err := json.Unmarshal([]byte(rawCode.SourceCode[1:len(rawCode.SourceCode)-1]), sourceCode); err != nil {
			return []*SourceCode{{
				Sources: Sources{"main.sol": &Contract{Content: rawCodes[0].SourceCode}},
			}}, nil
		}

		sourceCodes = append(sourceCodes, sourceCode)
	}

	return sourceCodes, nil
}

type Response struct {
	Status  string     `json:"status"`
	Message string     `json:"message"`
	Codes   []*RawCode `json:"result"`
}

type RawCode struct {
	SourceCode           string `json:"SourceCode"`
	Abi                  string `json:"ABI"`
	ContractName         string `json:"ContractName"`
	CompilerVersion      string `json:"CompilerVersion"`
	OptimizationUsed     string `json:"OptimizationUsed"`
	Runs                 string `json:"Runs"`
	ConstructorArguments string `json:"ConstructorArguments"`
	EVMVersion           string `json:"EVMVersion"`
	Library              string `json:"Library"`
	LicenseType          string `json:"LicenseType"`
	Proxy                string `json:"Proxy"`
	Implementation       string `json:"Implementation"`
	SwarmSource          string `json:"SwarmSource"`
	IsOneSource          bool
}

// SourceCodeFields
type SourceCode struct {
	Language string   `json:"language"`
	Sources  Sources  `json:"sources"`
	Settings Settings `json:"settings"`
}

type Sources map[string]*Contract

type Contract struct {
	Content string `json:"content"`
}

type Settings struct {
	Optimizer       *Optimizer      `json:"optimizer"`
	OutputSelection OutputSelection `json:"outputSelection"`
	Libraries       Libraries       `json:"libraries"`
}

type Optimizer struct {
	Enabled bool `json:"enabled"`
	Runs    int  `json:"runs"`
}

type OutputSelection map[string]map[string][]string

type Libraries interface{}
