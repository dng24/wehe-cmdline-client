// Parses and provides configurations for the Wehe command line client.
package config

import (
    "fmt"
    "strings"

    "gopkg.in/ini.v1"
)

// Configurations for the Wehe command line client
// configs are read in from the command line and from a .ini config file
type Config struct {
    // args from command line
    TestNames []string

    // args from ini config file
    ServerDisplay string
    NumServers int
    ExtraString string
    SendMobileStats bool
    Result bool
    ConfirmationReplays bool
    UseDefaultThresholds bool
    AreaThreshold int
    KS2PValueThreshold int
    LogLevel int
    UserConfigFile string
    TestsConfigFile string
    ServerCertFile string
    TestsDir string
    ResultsUIDir string
    ResultsLogDir string
    InfoFile string
}

// Creates a new Config object.
// testNames: names of the tests to run, delimitated by commas
// configPath: path to the .ini config file
// Returns a configuration struct or an error
func New(testNames *string, configPath *string) (Config, error) {
    // process command line arguments
    config := Config{}
    testNamesSlice := strings.Split(*testNames, ",")
    var nonEmptyStrings []string
    for _, testName := range testNamesSlice {
        s := strings.TrimSpace(strings.ToLower(testName))
        if s != "" {
            nonEmptyStrings = append(nonEmptyStrings, s)
        }
    }
    if len(nonEmptyStrings) == 0 {
        return config, fmt.Errorf("No test names entered.")
    }

    config.TestNames = nonEmptyStrings

    // process configs from the configuration file
    configFile, err := ini.Load(*configPath)
    if err != nil {
        return config, err
    }
    defaultSection := configFile.Section("")

    config.ServerDisplay, err = getString(defaultSection, "server_display")
    if err != nil {
        return config, err
    }

    // we are limited to 4 servers because MLab returns only 4 servers to choose from
    config.NumServers, err = getInt(defaultSection, "num_servers", 1, 4)
    if err != nil {
        return config, err
    }

    config.ExtraString, err = getString(defaultSection, "extra_string")
    if err != nil {
        return config, err
    }

    config.SendMobileStats, err = getBool(defaultSection, "send_mobile_stats")
    if err != nil {
        return config, err
    }

    config.Result, err = getBool(defaultSection, "result")
    if err != nil {
        return config, err
    }

    config.ConfirmationReplays, err = getBool(defaultSection, "confirmation_replays")
    if err != nil {
        return config, err
    }

    config.UseDefaultThresholds, err = getBool(defaultSection, "use_default_thresholds")
    if err != nil {
        return config, err
    }

    config.AreaThreshold, err = getInt(defaultSection, "area_threshold", 0, 100)
    if err != nil {
        return config, err
    }

    config.KS2PValueThreshold, err = getInt(defaultSection, "ks2pvalue_threshold", 0, 100)
    if err != nil {
        return config, err
    }

    config.LogLevel, err = getLogLevel(defaultSection, "log_level")
    if err != nil {
        return config, err
    }

    config.UserConfigFile, err = getString(defaultSection, "user_config_file")
    if err != nil {
        return config, err
    }

    config.TestsConfigFile, err = getString(defaultSection, "tests_config_file")
    if err != nil {
        return config, err
    }

    config.ServerCertFile, err = getString(defaultSection, "server_cert_file")
    if err != nil {
        return config, err
    }

    config.TestsDir, err = getString(defaultSection, "tests_dir")
    if err != nil {
        return config, err
    }

    config.ResultsUIDir, err = getString(defaultSection, "results_ui_dir")
    if err != nil {
        return config, err
    }

    config.ResultsLogDir, err = getString(defaultSection, "results_log_dir")
    if err != nil {
        return config, err
    }

    config.InfoFile, err = getString(defaultSection, "info_file")
    if err != nil {
        return config, err
    }

    return config, nil
}

// Gets a string from the config file.
// section: the section of the ini file that contains the key
// keyStr: the key
// Returns the value of the key or an error
func getString(section *ini.Section, keyStr string) (string, error) {
    key, err := section.GetKey(keyStr)
    if err != nil {
        return "", err
    }
    val := key.String()
    if val == "" {
        return "", fmt.Errorf("No value read from %s key", keyStr)
    }
    return val, nil
}

// Gets a log level from the config file.
// section: the section of the ini file that contains the key
// keyStr: the key
// Returns the integer value of the log level or an error
func getLogLevel(section *ini.Section, keyStr string) (int, error) {
    val, err := getString(section, keyStr)
    if err != nil {
        return -1, err
    }

    switch val {
    case "ui":
        return 0, nil
    case "wtf":
        return 1, nil
    case "error":
        return 2, nil
    case "warn":
        return 3, nil
    case "info":
        return 4, nil
    case "debug":
        return 5, nil
    default:
        return -1, fmt.Errorf("%s is not a log level. Choose from ui, wtf, error, warn, info, or debug.", val)
    }
}

// Gets an integer from the config file.
// section: the section of the ini file that contains the key
// keyStr: the key
// low: the lower bounds (inclusive) that the value should not go below
// high: the upper bounds (inclusive) that the value should not go above
// Returns the value or an error
func getInt(section *ini.Section, keyStr string, low int, high int) (int, error) {
    key, err := section.GetKey(keyStr)
    if err != nil {
        return -1, err
    }
    val, err := key.Int()
    if err != nil {
        return -1, fmt.Errorf("%s in %s key", err, keyStr)
    }
    if val < low || val > high {
        return -1, fmt.Errorf("%d is not a valid number for %s. Must be between %d and %d inclusive.", val, keyStr, low, high)
    }
    return val, nil
}

// Gets a boolean from the config file.
// section: the section of the ini file that contains the key
// keyStr: the key
//Returns the value or an error
func getBool(section *ini.Section, keyStr string) (bool, error) {
    key, err := section.GetKey(keyStr)
    if err != nil {
        return false, err
    }
    val, err := key.Bool()
    if err != nil {
        return false, fmt.Errorf("%s in %s key", err, keyStr)
    }
    return val, nil
}
