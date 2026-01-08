#!/bin/bash

# setup_path.sh - Automatically adds ~/.local/bin to PATH for various shells
# Supported: zsh, bash, fish

LOCAL_BIN="$HOME/.local/bin"
PATH_LINE="export PATH=\"\$HOME/.local/bin:\$PATH\""
FISH_PATH_LINE="fish_add_path \$HOME/.local/bin"

echo "Detecting shell and configuring PATH..."

# Create directory if it doesn't exist
mkdir -p "$LOCAL_BIN"

# Function to add to config if not present
add_to_config() {
    local config_file=$1
    local line=$2
    if [ -f "$config_file" ]; then
        if ! grep -Fq "$line" "$config_file"; then
            echo "" >> "$config_file"
            echo "# The Cloud CLI PATH" >> "$config_file"
            echo "$line" >> "$config_file"
            echo "Added to $config_file"
            return 0
        else
            echo "Found in $config_file, skipping."
            return 1
        fi
    fi
}

# 1. ZSH
add_to_config "$HOME/.zshrc" "$PATH_LINE"

# 2. BASH
add_to_config "$HOME/.bashrc" "$PATH_LINE"
add_to_config "$HOME/.bash_profile" "$PATH_LINE"
add_to_config "$HOME/.profile" "$PATH_LINE"

# 3. FISH
FISH_CONFIG="$HOME/.config/fish/config.fish"
if [ -d "$HOME/.config/fish" ]; then
    if [ ! -f "$FISH_CONFIG" ]; then
        touch "$FISH_CONFIG"
    fi
    add_to_config "$FISH_CONFIG" "$FISH_PATH_LINE"
fi

echo "PATH setup complete. Please restart your terminal or run:"
echo "   source ~/.zshrc (or your shell's config file)"
