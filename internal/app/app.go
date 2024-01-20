// Provides the main logic for Wehe.
package app

import (
    "wehe-cmdline-client/internal/config"
    "wehe-cmdline-client/internal/replay"
)

// Run the Wehe app.
// cfg: the configurations to run Wehe with
// Returns any errors
func Run(cfg config.Config) error {
    tests, err := replay.ParseTestJSON(cfg.TestsConfigFile, cfg.TestNames, cfg.TestsDir)
    if err != nil {
        return err
    }
    _ = tests
    return nil
}
