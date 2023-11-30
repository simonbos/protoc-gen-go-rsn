package examplersn_test

import (
	"fmt"
	"log"

	"github.com/simonbos/protoc-gen-go-rsn/example/examplersn"
)

func ExampleParseLogEntryParentResourceName() {
	resourceName := "organizations/abc"
	logEntryRsn, err := examplersn.ParseLogEntryParentResourceName(resourceName)
	if err != nil {
		log.Fatalf("error parsing LogEntry resource name: %v", err)
	}

	// Check 'Type' to access pattern-specific fields
	switch logEntryRsn.Type {
	case examplersn.ProjectLogEntryParentType:
		fmt.Println("Project Id:", logEntryRsn.ProjectId)
	case examplersn.OrganizationLogEntryParentType:
		fmt.Println("Organization Id:", logEntryRsn.OrganizationId)
	case examplersn.FolderLogEntryParentType:
		fmt.Println("Folder Id:", logEntryRsn.FolderId)
	case examplersn.BillingAccountLogEntryParentType:
		fmt.Println("Billing Account Id:", logEntryRsn.BillingAccountId)
	default:
		fmt.Println("Unknown LogEntry parent type")
	}

	outResourceName := logEntryRsn.ResourceName()
	if resourceName != outResourceName {
		log.Fatalf("got %v, want %v", outResourceName, resourceName)
	}
}

func ExampleParseLogEntryResourceName() {
	resourceName := "organisations/abc/logEntries/bef"
	logEntryRsn, err := examplersn.ParseLogEntryResourceName(resourceName)
	if err != nil {
		log.Fatalf("error parsing LogEntry resource name: %v", err)
	}

	fmt.Println("Log Entry Id:", logEntryRsn.LogEntryId)

	// Check 'Type' to access pattern-specific fields
	switch logEntryRsn.Parent.Type {
	case examplersn.ProjectLogEntryParentType:
		fmt.Println("Project Id:", logEntryRsn.Parent.ProjectId)
	case examplersn.OrganizationLogEntryParentType:
		fmt.Println("Organization Id:", logEntryRsn.Parent.OrganizationId)
	case examplersn.FolderLogEntryParentType:
		fmt.Println("Folder Id:", logEntryRsn.Parent.FolderId)
	case examplersn.BillingAccountLogEntryParentType:
		fmt.Println("Billing Account Id:", logEntryRsn.Parent.BillingAccountId)
	default:
		fmt.Println("Unknown LogEntry parent type")
	}

	outResourceName := logEntryRsn.ResourceName()
	if resourceName != outResourceName {
		log.Fatalf("got %v, want %v", outResourceName, resourceName)
	}
}
