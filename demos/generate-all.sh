#!/bin/bash

# Script to generate all VHS demo GIFs

echo "🎬 Generating JavaScript REPL demo GIFs..."
echo "Make sure VHS is installed: https://github.com/charmbracelet/vhs"
echo

# Check if VHS is available
if ! command -v vhs &> /dev/null; then
    echo "❌ VHS is not installed. Please install it first:"
    echo "   go install github.com/charmbracelet/vhs@latest"
    exit 1
fi

# Build the jesus server first
echo "🔨 Building jesus..."
cd .. && go build . && cd demos
if [ $? -ne 0 ]; then
    echo "❌ Failed to build jesus"
    exit 1
fi

echo "✅ jesus built successfully"
echo

# Generate each demo
demos=(
    "basic-repl.tape:Basic REPL Usage"
    "multiline-mode.tape:Multiline Mode"
    "slash-commands.tape:Slash Commands"
    "error-handling.tape:Error Handling"
    "multiline-flag.tape:Multiline Flag"
    "history-navigation.tape:History Navigation"
    "external-editor.tape:External Editor"
)

for demo_info in "${demos[@]}"; do
    IFS=':' read -r tape_file description <<< "$demo_info"
    
    if [ -f "$tape_file" ]; then
        echo "🎥 Generating: $description ($tape_file)"
        vhs < "$tape_file"
        
        if [ $? -eq 0 ]; then
            echo "✅ Generated successfully"
        else
            echo "❌ Failed to generate $tape_file"
        fi
        echo
    else
        echo "❌ Tape file not found: $tape_file"
    fi
done

echo "🎉 Demo generation complete!"
echo
echo "Generated GIFs:"
ls -la *.gif 2>/dev/null || echo "No GIF files found. Check for errors above."
