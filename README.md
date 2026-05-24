# universal-repo-vault

## Project Background and Idea

This project started from a problem I encountered while working on my small homelab. I set up a few small PCs as a k3s cluster to host lightweight automations and services. The plan is to manage everything in a single repository, using Kustomize for Kubernetes deployments and Ansible for machine setup.
There are already a few solutions that solve parts of this problem, and combining some of them would probably be enough. However, they all feel like too much hassle for my simple setup, and combining several tools increases the chance of making mistakes.
That is why I wanted to experiment a little and build a simple solution for storing, encrypting, and decrypting data in a repository.

## Goal

I want to create a simple CLI tool that can initialize its configuration, create a state lock file, update `.gitignore`, and eventually add Git hooks. After initialization, the tool should collect the contents of specified files in a repository, store them in an encrypted archive, and decrypt them on demand.

