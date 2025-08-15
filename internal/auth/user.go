package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user account with hashed password
type User struct {
	Username string `toml:"username"`
	Password string `toml:"password"` // Will be hashed after first load
	Role     string `toml:"role"`     // admin, user
	Created  string `toml:"created"`  // Creation timestamp
}

// UserConfig represents the structure of users.toml
type UserConfig struct {
	Users []User `toml:"users"`
}

// UserStore manages user authentication and storage
type UserStore struct {
	users    map[string]*User
	filePath string
}

// NewUserStore creates a new user store and loads users from the specified file
func NewUserStore(filePath string) (*UserStore, error) {
	store := &UserStore{
		users:    make(map[string]*User),
		filePath: filePath,
	}

	if err := store.loadUsers(); err != nil {
		return nil, fmt.Errorf("failed to load users: %w", err)
	}

	return store, nil
}

// loadUsers loads users from the TOML file and hashes passwords if needed
func (us *UserStore) loadUsers() error {
	// Check if file exists
	if _, err := os.Stat(us.filePath); os.IsNotExist(err) {
		// Create default admin user
		return us.createDefaultUser()
	}

	// Load existing file
	var config UserConfig
	if _, err := toml.DecodeFile(us.filePath, &config); err != nil {
		return fmt.Errorf("failed to parse users file: %w", err)
	}

	needsSave := false
	for i := range config.Users {
		user := &config.Users[i]

		// Check if password is already hashed (bcrypt hashes start with $2a$, $2b$, or $2y$)
		if !isHashedPassword(user.Password) {
			// Hash the plaintext password
			hashedPassword, err := hashPassword(user.Password)
			if err != nil {
				return fmt.Errorf("failed to hash password for user %s: %w", user.Username, err)
			}
			user.Password = hashedPassword
			needsSave = true
		}

		us.users[user.Username] = user
	}

	// Save back to file if we hashed any passwords
	if needsSave {
		return us.saveUsers(&config)
	}

	return nil
}

// createDefaultUser creates a default admin user if no users file exists
func (us *UserStore) createDefaultUser() error {
	// Generate a random password for security
	password, err := generateRandomPassword(12)
	if err != nil {
		return fmt.Errorf("failed to generate default password: %w", err)
	}

	hashedPassword, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash default password: %w", err)
	}

	defaultUser := User{
		Username: "admin",
		Password: hashedPassword,
		Role:     "admin",
		Created:  time.Now().Format("2006-01-02 15:04:05"),
	}

	config := UserConfig{
		Users: []User{defaultUser},
	}

	us.users["admin"] = &defaultUser

	if err := us.saveUsers(&config); err != nil {
		return err
	}

	// Print the generated password to stdout so admin can see it
	fmt.Printf("\n"+
		"=====================================\n"+
		"DEFAULT ADMIN USER CREATED\n"+
		"=====================================\n"+
		"Username: admin\n"+
		"Password: %s\n"+
		"=====================================\n"+
		"Please change this password by editing users.toml\n\n", password)

	return nil
}

// saveUsers saves the user configuration back to the TOML file
func (us *UserStore) saveUsers(config *UserConfig) error {
	file, err := os.Create(us.filePath)
	if err != nil {
		return fmt.Errorf("failed to create users file: %w", err)
	}
	defer file.Close()

	// Write header comment
	header := `# Staccato Users Configuration
# This file contains user accounts for authentication.
# Passwords will be automatically hashed when the server starts.
# To add a new user, add a new [[users]] section with username and password.
# To change a password, replace the hashed password with a new plaintext password.

`
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write users file header: %w", err)
	}

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode users to TOML: %w", err)
	}

	return nil
}

// Authenticate checks if the provided username and password are valid
func (us *UserStore) Authenticate(username, password string) bool {
	user, exists := us.users[username]
	if !exists {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	return err == nil
}

// GetUser returns a user by username (without password)
func (us *UserStore) GetUser(username string) *User {
	user, exists := us.users[username]
	if !exists {
		return nil
	}

	// Return copy without password
	return &User{
		Username: user.Username,
		Password: "", // Don't expose password hash
		Role:     user.Role,
		Created:  user.Created,
	}
}

// RegisterUser adds a new user to the store
func (us *UserStore) RegisterUser(username, password string) error {
	// Check if user already exists
	if _, exists := us.users[username]; exists {
		return fmt.Errorf("user already exists")
	}

	// Hash the password
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create new user
	newUser := User{
		Username: username,
		Password: hashedPassword,
		Role:     "user", // Default role
		Created:  time.Now().Format("2006-01-02 15:04:05"),
	}

	// Add to memory store
	us.users[username] = &newUser

	// Save to file
	return us.saveUsersToFile()
}

// saveUsersToFile saves the current users to the TOML file
func (us *UserStore) saveUsersToFile() error {
	// Convert users map back to slice for saving
	var usersList []User
	for _, user := range us.users {
		usersList = append(usersList, *user)
	}

	config := UserConfig{
		Users: usersList,
	}

	return us.saveUsers(&config)
} // hashPassword hashes a plaintext password using bcrypt
func hashPassword(password string) (string, error) {
	// Use cost factor 12 for good security/performance balance
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// isHashedPassword checks if a password string is already hashed
func isHashedPassword(password string) bool {
	// bcrypt hashes have a specific format: $2a$, $2b$, $2x$, or $2y$ followed by cost and salt
	return len(password) >= 4 &&
		password[0] == '$' &&
		password[1] == '2' &&
		(password[2] == 'a' || password[2] == 'b' || password[2] == 'x' || password[2] == 'y') &&
		password[3] == '$'
}

// generateRandomPassword generates a cryptographically secure random password
func generateRandomPassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Convert to hex string for readability
	return hex.EncodeToString(bytes)[:length], nil
}
