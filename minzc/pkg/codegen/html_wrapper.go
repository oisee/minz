package codegen

import (
	"fmt"
	"path/filepath"
	"strings"
)

// GenerateHTMLWrapper creates an HTML file that loads and runs the WASM module
func GenerateHTMLWrapper(wasmFile string, title string) string {
	if title == "" {
		title = "MinZ WebAssembly"
	}
	
	wasmFilename := filepath.Base(wasmFile)
	
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body {
            font-family: 'Courier New', monospace;
            background-color: #1e1e1e;
            color: #d4d4d4;
            margin: 20px;
        }
        h1 {
            color: #4ec9b0;
        }
        #output {
            background-color: #000;
            color: #00ff00;
            padding: 20px;
            border: 1px solid #4ec9b0;
            border-radius: 5px;
            min-height: 300px;
            white-space: pre;
            overflow-x: auto;
        }
        #controls {
            margin: 20px 0;
        }
        button {
            background-color: #4ec9b0;
            color: #1e1e1e;
            border: none;
            padding: 10px 20px;
            margin-right: 10px;
            border-radius: 3px;
            cursor: pointer;
            font-weight: bold;
        }
        button:hover {
            background-color: #3ea894;
        }
        .info {
            color: #808080;
            font-size: 0.9em;
            margin-top: 10px;
        }
    </style>
</head>
<body>
    <h1>%s</h1>
    <div id="controls">
        <button onclick="runProgram()">Run</button>
        <button onclick="clearOutput()">Clear</button>
    </div>
    <div id="output"></div>
    <div class="info">
        MinZ program compiled to WebAssembly • <a href="https://github.com/oisee/minz" style="color: #4ec9b0;">Learn more</a>
    </div>
    
    <script>
        let wasmInstance = null;
        const output = document.getElementById('output');
        
        function print(text) {
            output.textContent += text;
        }
        
        function clearOutput() {
            output.textContent = '';
        }
        
        async function loadWasm() {
            try {
                const response = await fetch('%s');
                const bytes = await response.arrayBuffer();
                
                const importObject = {
                    env: {
                        // Basic I/O functions
                        print_u8: (val) => print(val + ' '),
                        print_u16: (val) => print(val + ' '),
                        print_i8: (val) => print(val + ' '),
                        print_i16: (val) => print(val + ' '),
                        print_char: (val) => print(String.fromCharCode(val)),
                        print_string: (ptr, len) => {
                            const memory = wasmInstance.exports.memory;
                            const bytes = new Uint8Array(memory.buffer, ptr, len);
                            const str = new TextDecoder().decode(bytes);
                            print(str);
                        },
                        print_newline: () => print('\n'),
                        
                        // Memory management (stubs for now)
                        malloc: (size) => 0,
                        free: (ptr) => {},
                        
                        // System functions
                        halt: () => {
                            print('\n[Program halted]');
                            throw new Error('Program halted');
                        }
                    }
                };
                
                const { instance } = await WebAssembly.instantiate(bytes, importObject);
                wasmInstance = instance;
                
                print('[WASM module loaded successfully]\n\n');
                return instance;
            } catch (error) {
                print('[Error loading WASM: ' + error.message + ']\n');
                console.error(error);
                return null;
            }
        }
        
        async function runProgram() {
            clearOutput();
            
            if (!wasmInstance) {
                wasmInstance = await loadWasm();
            }
            
            if (wasmInstance && wasmInstance.exports.main) {
                try {
                    print('[Running MinZ program...]\n\n');
                    wasmInstance.exports.main();
                    print('\n\n[Program completed successfully]');
                } catch (error) {
                    if (error.message !== 'Program halted') {
                        print('\n[Runtime error: ' + error.message + ']');
                        console.error(error);
                    }
                }
            } else {
                print('[Error: No main function found in WASM module]');
            }
        }
        
        // Auto-run on page load
        window.addEventListener('load', runProgram);
    </script>
</body>
</html>`, title, title, wasmFilename)
}

// GenerateStandaloneHTML creates a self-contained HTML with embedded WASM
func GenerateStandaloneHTML(wasmBytes []byte, title string) string {
	if title == "" {
		title = "MinZ WebAssembly"
	}
	
	// Convert WASM bytes to base64
	base64Data := bytesToBase64(wasmBytes)
	
	// Replace the fetch with embedded data
	html := GenerateHTMLWrapper("embedded", title)
	html = strings.Replace(html, 
		`const response = await fetch('embedded');
                const bytes = await response.arrayBuffer();`,
		fmt.Sprintf(`const base64 = '%s';
                const binaryString = atob(base64);
                const bytes = new Uint8Array(binaryString.length);
                for (let i = 0; i < binaryString.length; i++) {
                    bytes[i] = binaryString.charCodeAt(i);
                }`, base64Data),
		1)
	
	return html
}

// bytesToBase64 converts bytes to base64 string
func bytesToBase64(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	result := ""
	
	for i := 0; i < len(data); i += 3 {
		var b1, b2, b3 byte
		b1 = data[i]
		
		if i+1 < len(data) {
			b2 = data[i+1]
		}
		if i+2 < len(data) {
			b3 = data[i+2]
		}
		
		result += string(base64Chars[b1>>2])
		result += string(base64Chars[((b1&0x03)<<4)|(b2>>4)])
		
		if i+1 < len(data) {
			result += string(base64Chars[((b2&0x0F)<<2)|(b3>>6)])
		} else {
			result += "="
		}
		
		if i+2 < len(data) {
			result += string(base64Chars[b3&0x3F])
		} else {
			result += "="
		}
	}
	
	return result
}