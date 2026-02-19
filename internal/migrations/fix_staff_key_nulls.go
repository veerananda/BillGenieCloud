package migrations

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"gorm.io/gorm"
)

// FixStaffKeyNulls fills null staff_key values with unique generated keys
func FixStaffKeyNulls(db *gorm.DB) error {
	// Get list of users with null staff_key
	type UserRecord struct {
		ID string
	}

	var users []UserRecord
	if err := db.Raw("SELECT id FROM users WHERE staff_key IS NULL OR staff_key = ''").Scan(&users).Error; err != nil {
		// Table might not exist yet, which is fine
		fmt.Printf("⚠️  Users table doesn't exist yet or error querying: %v\n", err)
		return nil
	}

	if len(users) == 0 {
		fmt.Printf("✅ No users with null staff_key\n")
		return nil
	}

	fmt.Printf("🔍 Found %d users with null staff_key, generating unique keys...\n", len(users))

	// Generate unique keys for each user
	for _, user := range users {
		// Generate a random 16-byte key and base64 encode it
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			fmt.Printf("⚠️  Failed to generate random key for user %s: %v\n", user.ID, err)
			continue
		}

		staffKey := base64.URLEncoding.EncodeToString(bytes)[:16] // Keep it short and URL-safe

		// Update the user with the generated key
		result := db.Exec("UPDATE users SET staff_key = ? WHERE id = ?", staffKey, user.ID)
		if result.Error != nil {
			fmt.Printf("⚠️  Failed to update user %s: %v\n", user.ID, result.Error)
		}
	}

	// Verify the fix
	var countAfter int64
	db.Raw("SELECT COUNT(*) FROM users WHERE staff_key IS NULL OR staff_key = ''").Scan(&countAfter)
	if countAfter == 0 {
		fmt.Printf("✅ Updated %d users with generated staff_key values\n", len(users))
	} else {
		fmt.Printf("⚠️  Still %d users with null staff_key after fix\n", countAfter)
	}

	return nil
}
