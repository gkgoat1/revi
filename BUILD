load("@io_bazel_rules_go//go:def.bzl", "go_binary")
go_binary(name = "revi", srcs = glob(["*.go"]),deps = ["//pkg:pkg","//fetch:fetch"],visibility = ["//visibility:public"],    gc_linkopts = [
        "-linkmode",
        "external",
        "-extldflags",
        "-static",
    ])
filegroup(name = "revi-final",srcs = [":revi"], output_group="static")