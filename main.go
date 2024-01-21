// The main file of the Wehe app.
package main

import (
    "flag"
    "fmt"
    "os"

    "wehe-cmdline-client/internal/app"
    "wehe-cmdline-client/internal/config"
)

func main() {
    // parse command line arguments
    replaySubcommand := flag.NewFlagSet("replay", flag.ExitOnError)
    testNames := replaySubcommand.String("n", "", "name of the tests to run, comma-delimitated (required argument; see below for list of tests)")
    configFile := replaySubcommand.String("c", "res/config/config.ini", "")

    //updateSubcommand := flag.NewFlagSet("update", flag.ExitOnError)
    // TODO: finish update subcommand

    for _, arg := range os.Args {
        if arg == "-h" || arg == "--help" {
            //print usage
            os.Exit(0)
        }
        if arg == "-v" || arg == "--version" {
            //print version
            os.Exit(0)
        }
    }

    if len(os.Args) < 2 {
        fmt.Println("\"replay\" or \"update\" command expected")
        os.Exit(1)
    }

    switch os.Args[1] {
    case "replay":
        replaySubcommand.Parse(os.Args[2:])
    case "update":
    default:
        fmt.Println("\"replay\" or \"update\" command expected")
        os.Exit(1)
    }

    // read in wehe configs
    config, err := config.New(testNames, configFile)
    if err != nil {
        fmt.Printf("Unable to process configuration file %s: %s\n", *configFile, err)
        os.Exit(1)
    }

    // run the app
    err = app.Run(config)
    if err != nil {
        fmt.Print(err)
        os.Exit(1)
    }
}
