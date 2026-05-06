package cmd

// banner is the Purr Request mascot, shown at the top of `bitrise-cli`
// and `bitrise-cli --help`. Pure ASCII so it renders correctly through
// pagers, log files, and bare terminals; no ANSI codes — cobra's help
// text path doesn't go through our color profile.
//
// Leading newline so it doesn't run into cobra's "Long:" label.
const banner = `
                  _______
                 /  o  o  \           Purr Request
                ;    \_/   ;          --------------
                 \        /
                  '------'            Rocket-powered cat
                __|        |__        mascot for the
               |   *  *  *  |--==>>   Bitrise platform CLI.
               |______________|
                  /\      /\
                 '  '    '  '
`
