package config

import (
    "fmt"
    "strconv"
    "testing"

    "gopkg.in/ini.v1"
)

const (
    keyValPair = `
    %s = %s
    %s = %s
    %s = %s
    `
)

func setup(t *testing.T, key1 string, val1 string, key2 string, val2 string, key3 string, val3 string) *ini.Section {
    config, err := ini.Load([]byte(fmt.Sprintf(keyValPair, key1, val1, key2, val2, key3, val3)))
    if err != nil {
        t.Fatal(err)
    }

    section, err := config.GetSection("")
    if err != nil {
        t.Fatal(err)
    }

    return section
}

func TestGetString(t *testing.T) {
    key1 := "server_display"
    val1 := "wehe4.meddle.mobi"
    key2 := "mlab_servers_url"
    val2 := ""
    section := setup(t, key1, val1, key2, val2, "filler", "filler")

    // Test case 1: Key exists, value is not empty
	result, err := getString(section, key1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := val1
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test case 2: Key does not exist
	_, err = getString(section, "nonexistent_key")
	if err == nil {
		t.Error("Expected error for nonexistent key, but got none.")
	} else {
		expectedError := "error when getting key of section \"DEFAULT\": key \"nonexistent_key\" not exists"
		if fmt.Sprint(err) != expectedError {
			t.Errorf("Expected error '%s', got '%v'", expectedError, err)
		}
	}

	// Test case 3: Key exists, value is empty
	_, err = getString(section, key2)
	if err == nil {
		t.Error("Expected error for empty value, but got none.")
	} else {
		expectedError := fmt.Sprintf("No value read from %s key", key2)
		if fmt.Sprint(err) != expectedError {
			t.Errorf("Expected error '%s', got '%v'", expectedError, err)
		}
	}
}

func TestGetLogLevel(t *testing.T) {
    key1 := "log_level"
    val1 := "warn"
    key2 := "log_level2"
    val2 := "bad_log_level"
    section := setup(t, key1, val1, key2, val2, "filler", "filler")

    result, err := getLogLevel(section, key1)
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }
    expected := 3
    if result != expected {
        t.Errorf("Expected '%d', got '%d'", expected, result)
    }

    _, err = getLogLevel(section, key2)
    if err == nil {
        t.Error("Expected error for bad logging error, but got none.")
    } else {
        expectedError := fmt.Sprintf("%s is not a log level. Choose from ui, wtf, error, warn, info, or debug.", val2)
        if fmt.Sprint(err) != expectedError {
            t.Errorf("Expected error '%s', got '%v'", expectedError, err)
        }
    }
}

func TestGetInt(t *testing.T) {
    key1 := "side_channel_port"
    val1 := "40000"
    key2 := "result_port"
    val2 := "asdf"
    key3 := "area_threshold"
    val3 := "110"
    section := setup(t, key1, val1, key2, val2, key3, val3)

	// Test case 1: Key exists, value is an integer within the specified range
	result, err := getInt(section, key1, 0, 65535)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := 40000
	if result != expected {
		t.Errorf("Expected %d, got %d", expected, result)
	}

	// Test case 2: Key exists, value is not an integer
	_, err = getInt(section, key2, 0, 65535)
	if err == nil {
		t.Error("Expected error for non-integer value, but got none.")
	} else {
		expectedError := fmt.Sprintf("strconv.ParseInt: parsing \"asdf\": invalid syntax in %s key", key2)
		if fmt.Sprint(err) != expectedError {
			t.Errorf("Expected error '%s', got '%v'", expectedError, err)
		}
	}

	// Test case 3: Key exists, value is an integer outside the specified range
	_, err = getInt(section, key3, 0, 100)
	if err == nil {
		t.Error("Expected error for out-of-range value, but got none.")
	} else {
		expectedError := fmt.Sprintf("%s is not a valid number for %s. Must be between 0 and 100 inclusive.", val3, key3)
		if fmt.Sprint(err) != expectedError {
			t.Errorf("Expected error '%s', got '%v'", expectedError, err)
		}
	}
}

func TestGetBool(t *testing.T) {
    key1 := "result"
    val1 := "true"
    key2 := "send_mobile_stats"
    val2 := "True"
    key3 := "confirmation_replays"
    val3 := "asdf"
    section := setup(t, key1, val1, key2, val2, key3, val3)

    result, err := getBool(section, key1)
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }
    expected := true
    if result != expected {
        t.Errorf("Expected %t, got %t", expected, result)
    }

    result, err = getBool(section, key2)
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }
    expected = true
    if result != expected {
        t.Errorf("Expected %t, got %t", expected, result)
    }

    _, err = getBool(section, key3)
    if err == nil {
        t.Error("Expected error for non-boolean value, but got none.")
    } else {
        expectedError := fmt.Sprintf("%s in %s key", "parsing \"asdf\": invalid syntax", key3)
        if fmt.Sprint(err) != expectedError {
            t.Errorf("Expected error '%s', got '%v'", expectedError, err)
        }
    }
}

func TestNew(t *testing.T) {
    testNames := "test1,Test2 , TEST3"
    configFile := "testdata/test.ini"
    config, err := New(&testNames, &configFile)
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }

    if len(config.TestNames) != 3 {
        t.Errorf("Expected 3, got %d", len(config.TestNames))
    }

    for i := 0; i < 3; i++ {
        if config.TestNames[i] != "test" + strconv.Itoa(i + 1) {
            t.Errorf("Expected %s, got %s", "test" + strconv.Itoa(i + 1), config.TestNames[i])
        }
    }

    if config.ResultPort != 56566 {
        t.Errorf("Expected 56566, got %d", config.ResultPort)
    }

    if config.ConfirmationReplays != true {
        t.Errorf("Expected false, got %t", config.ConfirmationReplays)
    }

    if config.LogLevel != 0 {
        t.Errorf("Expected 0, got %d", config.LogLevel)
    }

    if config.ServerDisplay != "wehe4.meddle.mobi" {
        t.Errorf("Expected wehe4.meddle.mobi, got %s", config.ServerDisplay)
    }

    testNames = "test1"
    configFile = "nonexistent/path"
    _, err = New(&testNames, &configFile)
    if err == nil {
        t.Error("Expected error for non-existent config file path, but got none.")
    } else {
        expectedError := "open nonexistent/path: no such file or directory"
        if fmt.Sprint(err) != expectedError {
            t.Errorf("Expected error '%s', got '%v'", expectedError, err)
        }
    }
}
