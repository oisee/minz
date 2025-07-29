#!/usr/bin/env python3
import os
import subprocess
import sys

def test_compilation():
    examples_dir = "../examples"
    minzc_path = "./minzc"
    
    # Change to minzc directory
    original_dir = os.getcwd()
    os.chdir("minzc")
    
    # Check if minzc exists
    if not os.path.exists(minzc_path):
        print("Error: minzc not found. Run 'cd minzc && make build' first")
        os.chdir(original_dir)
        return
    
    files = sorted([f for f in os.listdir(examples_dir) if f.endswith('.minz')])
    
    successful = 0
    failed = 0
    results = []
    
    for file in files:
        file_path = os.path.join(examples_dir, file)
        cmd = [minzc_path, file_path, "-o", "/tmp/test.a80"]
        
        try:
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
            if result.returncode == 0:
                successful += 1
                results.append(f"{file}: Successfully compiled to /tmp/test.a80")
            else:
                failed += 1
                # Extract just the error message, not the full output
                error_lines = result.stderr.strip().split('\n')
                error_msg = None
                for line in error_lines:
                    if "Error:" in line:
                        error_msg = line
                        break
                if not error_msg:
                    error_msg = error_lines[-1] if error_lines else "Unknown error"
                results.append(f"{file}: {error_msg}")
        except subprocess.TimeoutExpired:
            failed += 1
            results.append(f"{file}: Timeout")
        except Exception as e:
            failed += 1
            results.append(f"{file}: Exception: {e}")
    
    # Print results
    for result in results:
        print(result)
    
    print()
    print(f"Total files: {len(files)}")
    print(f"Successful: {successful}")
    print(f"Failed: {failed}")
    print(f"Success rate: {successful}/{len(files)} ({successful/len(files)*100:.1f}%)")
    
    # Save detailed results
    with open("/tmp/compilation_results.txt", "w") as f:
        f.write("\n".join(results) + "\n")
    
    # Restore original directory
    os.chdir(original_dir)

if __name__ == "__main__":
    test_compilation()