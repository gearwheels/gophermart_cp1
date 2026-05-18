package model

import "testing"

func TestTableNames(t *testing.T) {
	if (User{}).TableName() != "users" {
		t.Fatalf("unexpected users table name")
	}
	if (Order{}).TableName() != "orders" {
		t.Fatalf("unexpected orders table name")
	}
	if (Withdrawal{}).TableName() != "withdrawals" {
		t.Fatalf("unexpected withdrawals table name")
	}
}

func TestOrderStatuses(t *testing.T) {
	if OrderStatusNew == "" || OrderStatusProcessed == "" {
		t.Fatalf("statuses must not be empty")
	}
}

