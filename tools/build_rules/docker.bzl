def _dockerfile_image_impl(ctx):
    """Implementation of the dockerfile_image rule."""
    # Declare the output file for the image tarball.

    all_build_args = []
    all_input_files = [ctx.file.dockerfile]

    if ctx.file.dockerignore:
        # The source file provided by the user.
        source_ignore_file = ctx.file.dockerignore
        linked_ignore_file = ctx.actions.declare_file(".dockerignore")

        # Create an action that symlinks the user's file to ".dockerignore".
        ctx.actions.symlink(
            output=linked_ignore_file,
            target_file=source_ignore_file,
        )

        # Add the source ignore file and the symlink to the inputs of our
        # main 'docker build' action to ensure it's created before the build runs.
        all_input_files.append(source_ignore_file)
        all_input_files.append(linked_ignore_file)

    # Process regular string build arguments.
    # We quote the value to handle spaces and special characters safely.
    for key, value in ctx.attr.buildargs.items():
        all_build_args.append("--build-arg {}='{}'".format(key, value))

    # Process file-based build arguments from the 'buildargs_files' attribute.
    # ctx.attr.buildargs_files gives us a dictionary of { "arg_name": <target> }
    for key, target in ctx.attr.buildargs_files.items():
        files = " ".join([f.path for f in target.files.to_list()])
        all_build_args.append("--build-arg {}='{}'".format(key, files))
        for file in target.files.to_list():
            all_input_files.append(file)

    out_flags = ""
    outputs = []
    post_action = ""
    if ctx.attr.tag:
        tag_file = ctx.actions.declare_file(ctx.label.name + "_tag.txt")
        outputs.append(tag_file)
        out_flags += "--tag {tag}:$WORKSPACE_HASH ".format(
            tag=ctx.attr.tag,
        )
        post_action += 'echo "{tag}:$WORKSPACE_HASH" > {output_path}\n'.format(
            tag=ctx.attr.tag,
            output_path=tag_file.path,
        )
    if ctx.attr.tar:
        tar_file = ctx.actions.declare_file(ctx.label.name + ".tar")
        outputs.append(tar_file)
        out_flags += "--output type=tar,dest={output_path} ".format(
           output_path=tar_file.path,
        )

    # Construct the final docker build command.
    command = """
    sg docker << 'EOF'
        set -e
        export DOCKER_BUILDKIT=1
        export WORKSPACE_HASH=$(grep STABLE_WORKSPACE_HASH {status} | awk '{{print $2}}')
        docker buildx build --target {target} -f {dockerfile} {out_flags} {build_args} .
        {post_action}
EOF
    """.format(
        status=ctx.info_file.path,
        target=ctx.attr.target,
        dockerfile=ctx.file.dockerfile.path,
        out_flags=out_flags,
        build_args=" ".join(all_build_args),
        post_action=post_action,
    )

    no_cache = "0"

    # Create the shell action to execute the command.
    ctx.actions.run_shell(
        outputs=outputs,
        inputs=all_input_files,
        command=command,
        progress_message="Building Docker image %s" % ctx.label,
        execution_requirements = {
            "no-sandbox": "1",
            "no-remote": "1",
            # "no-cache": "1",
            "local": "1",
        }
    )

    # Return the output file as the result of this rule.
    return [DefaultInfo(files=depset(outputs))]

dockerfile_image = rule(
    implementation=_dockerfile_image_impl,
    attrs={
        "dockerfile": attr.label(
            mandatory=True,
            allow_single_file=True,
            doc="The Dockerfile to build.",
        ),
        "dockerignore": attr.label(
            allow_single_file=True,
            doc="The .dockerignore file for this build. Will be used as the root .dockerignore.",
        ),
        "target": attr.string(
            mandatory=True,
            doc="The target stage to build within a multi-stage Dockerfile.",
        ),
        "buildargs": attr.string_dict(
            doc="A dictionary of build arguments with string literal values.",
        ),
        "buildargs_files": attr.string_keyed_label_dict(
            doc="A dictionary mapping build argument names to file targets (labels).",
        ),
        "tag": attr.string(
            mandatory=False,
            doc="If set, the docker image will be saved to this tag, with a workspace specific suffix."
        ),
        "tar": attr.bool(
            default=False,
            doc="If set, the docker image will be saved as a tar file."
        )
    },
)