import os
import re

def replace_includes_in_file(file_path):
    """
    Replaces include statements in a given file, except for includes
    ending in .pb.h.

    Args:
        file_path (str): The path to the file to be processed.
    """
    prefixes = ['apps', 'io', 'proc', 'proxy', 'rpc', 'serr', 'sigmap', 'threadpool', 'user', 'util']

    try:
        with open(file_path, 'r') as f:
            original_content = f.read()
    except (IOError, OSError) as e:
        print(f"Error reading file {file_path}: {e}")
        return

    content = original_content

    def replacement_logic(match):
        """
        Determines if a replacement should happen based on the matched include path.
        """
        # The first captured group is the path inside the angle brackets.
        # e.g., for '#include <apps/some/file.h>', group(1) is 'apps/some/file.h'
        include_path = match.group(1)

        # If the included file path ends with .pb.h, do not change the line.
        if include_path.endswith('.pb.h'):
            return f'#include "{include_path}"'
        else:
            # Otherwise, perform the replacement.
            return f'#include "cpp/{include_path}"'

    for prefix in prefixes:
        # This pattern finds '#include <PREFIX/...>' and captures the 'PREFIX/...' part.
        pattern = re.compile(r'#include <(' + prefix + r'/[^>]+)>')
        content = pattern.sub(replacement_logic, content)

    # To avoid unnecessary file operations, only write back to the file if the
    # content has actually changed.
    if content != original_content:
        try:
            with open(file_path, 'w') as f:
                f.write(content)
            print(f"Modified: {file_path}")
        except (IOError, OSError) as e:
            print(f"Error writing to file {file_path}: {e}")
    else:
        print(f"No changes needed in: {file_path}")


def main():
    """
    Walks through the current directory and its subdirectories
    and processes all .cc and .h files.
    """
    for root, _, files in os.walk('.'):
        for file in files:
            if file.endswith(('.cc', '.h')):
                file_path = os.path.join(root, file)
                replace_includes_in_file(file_path)

if __name__ == '__main__':
    main()
