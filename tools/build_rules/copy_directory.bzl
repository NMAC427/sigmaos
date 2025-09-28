"""A rule to create a hermetic directory tree from a set of source files."""

def _copy_directory_impl(ctx):
    """Implementation for the copy_directory rule."""
    output_dir = ctx.actions.declare_directory(ctx.attr.out_dir)

    commands = ["set -e"]
    all_srcs = []

    # Process files from the 'mappings' attribute. This is the primary logic.
    for dest_dir_str, files in ctx.attr.mappings.items():
        for src_file in files.files.to_list():
            all_srcs.append(src_file)

            # --- Robust Path Construction Logic ---
            # 1. Normalize the destination directory string by removing leading/trailing slashes.
            #    This handles user inputs like "/kernel", "kernel/", and "kernel" identically.
            clean_dest_dir = dest_dir_str.strip('/')

            # 2. Combine the destination directory and the source file's basename.
            if clean_dest_dir:
                # If the cleaned dir is not empty (e.g., "kernel"), prepend it.
                # Result: "kernel/hello_wasm"
                dest_path_in_output_dir = clean_dest_dir + "/" + src_file.basename
            else:
                # If the cleaned dir is empty (input was "/" or ""), place the file at the root.
                # Result: "some_root_file.txt"
                dest_path_in_output_dir = src_file.basename

            # 3. Construct the full final path for the shell command.
            full_dest_path = output_dir.path + "/" + dest_path_in_output_dir
            # --- End of Path Logic ---

            # 4. Generate commands to create the directory and copy the file.
            commands.append("mkdir -p $(dirname %s)" % full_dest_path)
            commands.append("cp %s %s" % (src_file.path, full_dest_path))

    # If no inputs were provided at all, we can stop.
    if not all_srcs:
        # It's good practice to handle this case, though an empty directory is also fine.
        return [DefaultInfo(files = depset())]

    # Join all generated commands into a single shell script.
    script = " && ".join(commands)

    ctx.actions.run_shell(
        inputs = all_srcs,
        outputs = [output_dir],
        command = script,
        progress_message = "Assembling directory %s" % output_dir.short_path,
    )

    return [DefaultInfo(files = depset([output_dir]))]

copy_directory = rule(
    implementation = _copy_directory_impl,
    attrs = {
        "out_dir": attr.string(
            doc = "The name of the output directory.",
            mandatory = True,
        ),
        "mappings": attr.string_keyed_label_dict(
            doc = """A dictionary mapping a destination directory to a label (like a filegroup)
                 within the output directory. All files from the label will be copied into
                 the specified directory, preserving their basenames.""",
            allow_files = True,
        ),
        "srcs": attr.label_list(
            doc = "A list of files to include, using their original paths (modified by strip_prefix).",
            allow_files = True,
        ),
        "strip_prefix": attr.string(
            doc = "A prefix to remove from the paths of 'srcs' files.",
        ),
    },
)