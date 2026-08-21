use clap::{Parser, Subcommand};

/// A demo CLI to exercise the YKC smoke engine.
#[derive(Parser)]
#[command(version, about, long_about = None)]
struct Cli {
    /// Print extra info
    #[arg(long)]
    verbose: bool,

    #[command(subcommand)]
    command: Command,
}

#[derive(Subcommand)]
enum Command {
    /// Greet someone
    Greet {
        /// Who to greet
        name: String,
    },
    /// Sum a list of integers
    Sum {
        /// Numbers to sum
        nums: Vec<i64>,
    },
}

fn main() {
    let cli = Cli::parse();
    match cli.command {
        Command::Greet { name } => {
            if cli.verbose {
                println!("verbose: greeting");
            }
            println!("Hello, {name}!");
        }
        Command::Sum { nums } => {
            let total: i64 = nums.iter().sum();
            println!("{total}");
        }
    }
}
