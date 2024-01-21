// Provides the main logic for Wehe.
package app

import (
    "bufio"
    "fmt"
    "math/rand"
    "os"
    "strconv"
    "time"
    "unicode"

    "wehe-cmdline-client/internal/config"
    "wehe-cmdline-client/internal/replay"
)

const (
    charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    userIDLength = 10
    cmdlineUserIDFirstChar = "@"
)

// Run the Wehe app.
// cfg: the configurations to run Wehe with
// Returns any errors
func Run(cfg config.Config) error {
    //history count
    userID, testID := readUserConfig(cfg.UserConfigFile)
    fmt.Println(userID, testID)

    tests, err := replay.ParseTestJSON(cfg.TestsConfigFile, cfg.TestNames, cfg.TestsDir)
    if err != nil {
        return err
    }
    _ = tests

    //set up servers / certs

    //flip coin

    //run replays

    //get results
    return nil
}


//TODO: if above gets too long, move below to app.utils file


// Retrieves the user ID and test ID from a file. Creates new ones if file cannot be opened or
// config is invalid.
// userConfigFile: path of the file containing user ID and test ID
// Returns the user ID and test ID
func readUserConfig(userConfigFile string) (string, int) {
    file, err := os.Open(userConfigFile)
    if err != nil {
        return generateUserID(userIDLength), 0
    }
    fileScanner := bufio.NewScanner(file)
    fileScanner.Split(bufio.ScanLines)

    // first line of file is user ID
    fileScanner.Scan()
    userID := fileScanner.Text()
    // second line of file is test ID
    fileScanner.Scan()
    testID, err := strconv.Atoi(fileScanner.Text())
    if err != nil || !isUserIDValid(userID) || !isTestIDValid(testID) {
        userID = generateUserID(userIDLength)
        testID = 0
    }

    file.Close()

    return userID, testID
}

// Checks if a user ID is valid. A user ID is valid if it contains ten alphanumeric characters.
// However, unlike the iOS and Android apps, the first character is a '@' for the command line
// client.
// userID: the user ID to validate
// Returns true if user ID is valid; false otherwise
func isUserIDValid(userID string) bool {
    if len(userID) == userIDLength && string(userID[0]) == cmdlineUserIDFirstChar {
        for i := 1; i < userIDLength; i++ {
            if !unicode.IsLetter(rune(userID[i])) && !unicode.IsNumber(rune(userID[i])) {
                return false
            }
        }
        return true
    }
    return false
}

// Checks if a test ID is valid. A test ID is valid if it is a non-negative integer.
// testID: the test ID to validate
// Returns true if test ID is valid; false otherwise
func isTestIDValid(testID int) bool {
    return testID > -1
}

// Generates a user ID of random alphanumeric characters, with the first character as a '@'.
// length: total length of the user ID, including the '@'.
// Returns the user ID
func generateUserID(length int) string {
	rand.Seed(time.Now().UnixNano())

	result := make([]byte, length - 1)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}

	return cmdlineUserIDFirstChar + string(result)
}
