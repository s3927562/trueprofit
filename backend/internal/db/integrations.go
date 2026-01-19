package db

import "os"

func TransactionsTableName() string {
	return os.Getenv("TRANSACTIONS_TABLE")
}

func IntegrationsTableName() string {
	return os.Getenv("INTEGRATIONS_TABLE")
}

func OAuthStateTableName() string {
	return os.Getenv("OAUTH_STATE_TABLE")
}

func ShopToUserTableName() string {
	return os.Getenv("SHOP_TO_USER_TABLE")
}

func UsersTableName() string {
	return os.Getenv("USERS_TABLE")
}
