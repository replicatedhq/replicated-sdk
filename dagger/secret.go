package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"

	onepassword "github.com/1password/onepassword-sdk-go"
)

type Vault string

const (
	VaultDeveloperAutomation           Vault = "ikfulaksdrbqtjgybu2vkcggeu"
	VaultDeveloperAutomationProduction Vault = "4r7lasfjeevrao4qi4wsqgnn6e"
)

func mustGetItemUUID(ctx context.Context, opServiceAccount *dagger.Secret, itemName string, vault Vault) string {
	opServiceAccountPlaintext, err := opServiceAccount.Plaintext(ctx)
	if err != nil {
		panic(err)
	}
	client, err := onepassword.NewClient(ctx,
		onepassword.WithServiceAccountToken(opServiceAccountPlaintext),
		onepassword.WithIntegrationInfo("Dagger Workflow", "v0.0.1"),
	)

	items, err := client.Items.ListAll(ctx, string(vault))
	if err != nil {
		panic(err)
	}

	for {
		item, err := items.Next()
		if err != nil {
			if err == onepassword.ErrorIteratorDone {
				panic(fmt.Errorf("item %s not found", itemName))
			}
			panic(err)
		}

		if item.Title == itemName {
			return item.ID
		}
	}
}

func mustGetSecretAsPlaintext(ctx context.Context, opServiceAccount *dagger.Secret, itemName string, field string, vault Vault) string {
	secret := mustGetSecret(ctx, opServiceAccount, itemName, field, vault)

	pt, err := secret.Plaintext(ctx)
	if err != nil {
		panic(err)
	}

	return pt
}

func mustGetSecret(ctx context.Context, opServiceAccount *dagger.Secret, itemName string, field string, vault Vault) *dagger.Secret {
	opItemUUID := mustGetItemUUID(ctx, opServiceAccount, itemName, vault)

	opServiceAccountPlaintext, err := opServiceAccount.Plaintext(ctx)
	if err != nil {
		panic(err)
	}
	client, err := onepassword.NewClient(ctx,
		onepassword.WithServiceAccountToken(opServiceAccountPlaintext),
		onepassword.WithIntegrationInfo("Dagger Workflow", "v0.0.1"),
	)
	if err != nil {
		panic(err)
	}

	onePasswordURI := fmt.Sprintf("op://%s/%s/%s", vault, opItemUUID, field)
	item, err := client.Secrets.Resolve(context.Background(), onePasswordURI)
	if err != nil {
		panic(fmt.Errorf("failed to get field %s from item %s: %w", field, itemName, err))
	}

	return dagger.Connect().SetSecret(itemName, item)
}

func mustGetNonSensitiveSecret(ctx context.Context, opServiceAccount *dagger.Secret, itemName string, field string, vault Vault) string {
	opItemUUID := mustGetItemUUID(ctx, opServiceAccount, itemName, vault)

	opServiceAccountPlaintext, err := opServiceAccount.Plaintext(ctx)
	if err != nil {
		panic(err)
	}
	client, err := onepassword.NewClient(ctx,
		onepassword.WithServiceAccountToken(opServiceAccountPlaintext),
		onepassword.WithIntegrationInfo("Dagger Workflow", "v0.0.1"),
	)

	onePasswordURI := fmt.Sprintf("op://%s/%s/%s", vault, opItemUUID, field)
	item, err := client.Secrets.Resolve(context.Background(), onePasswordURI)
	if err != nil {
		panic(fmt.Errorf("failed to get field %s from item %s: %w", field, itemName, err))
	}

	return item
}
