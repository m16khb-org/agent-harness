package main

import "agent-harness/cmd/harness/contractcli"

type CompatibilityContract = contractcli.CompatibilityContract

func configureContractCLI() {
	contractcli.MCPTools = mcpTools
}

func runContract(args []string) error {
	configureContractCLI()
	return contractcli.Run(args)
}

func compatibilityContract() CompatibilityContract {
	configureContractCLI()
	return contractcli.BuildCompatibilityContract()
}
