/*
<!--
Copyright (c) 2019 Christoph Berger. Some rights reserved.

Use of the text in this file is governed by a Creative Commons Attribution Non-Commercial
Share-Alike License that can be found in the LICENSE.txt file.

Use of the code in this file is governed by a BSD 3-clause license that can be found
in the LICENSE.txt file.

The source code contained in this file may import third-party source code
whose licenses are provided in the respective license files.
-->

<!--
NOTE: The comments in this file are NOT godoc compliant. This is not an oversight.

Comments and code in this file are used for describing and explaining a particular topic to the reader. While this file is a syntactically valid Go source file, its main purpose is to get converted into a blog article. The comments were created for learning and not for code documentation.
-->

+++
title = "Distributed Computing With Dried, Salted Cod Fish, WASM, And (Tiny)Go"
description = "How to run a Go WASM binary on the distributed computing network Bacalhau."
author = "Christoph Berger"
images = ["media/ricardo-resende-mlzRoZqv_zM-unsplash-1200.jpg"]
email = "chris@appliedgo.net"
date = "2024-03-02"
draft = "false" 
categories = ["Distributed Computing"]
tags = ["wasm", "big data", "data science"]
articletypes = ["How-To Guide"]

+++	

You don't need monstrous software orchestration systems for collecting information from distributed data sources. Here is an easy way of sending a Go binary to where the data is.

<!--more-->

![An image of a cod in the ocean](ricardo-resende-mlzRoZqv_zM-unsplash-1200.jpg)

## Data is everywhere... but never where you need it

Heaps of gold.

That's what your data is.

The constant streams of log data. The scientific data collected from sensors, cameras, or the particle collider in your backyard. The activity data your security systems accumulates continuously in all your company's subsidiaries around the world.

There is only a *tiny* problem. The data is scattered across remote data centers. Moving huge amounts of data is difficult and expensive. How can you process data that's distributed around the world?

## Compute over data

Instead of sending all the data to one central point for processing, **send the processing jobs to where the data is.**

Data processing results are typically many orders of magnitude smaller than the original data and can be collected much easier and faster than moving all the raw data across the globe to one location.

Compute over data, or CoD, embraces this idea. A CoD system sends compute jobs to remote nodes, where the jobs process the data and send the results back to the originating node (or store the data somewhere to collect it later).

## Software orchestration can be overkill

If you do not already run a distributed Kubernetes supercluster, you'll want to avoid the huge overhead of setting up complex software orchestration systems with all their strings attached. 

You need a distributed data processing system that is easy to set up and maintain while being efficient and secure.

## Bacalhau specializes on orchestrating distributed batch jobs

[Bacalhau][Bacalhau] is a CoD system makes distributed data processing ridiculously easy. In a nutshell, you – 

- set up a swarm of Bacalhau nodes,
- create data processing jobs as Docker containers or WASM binaries,
- send those jobs to every node that hosts the data to process,
- collect the results.

Side note 1: Bacalhau is written in Go.

Side node 2: Bacalhau is the Portuguese name of dried, salted cod fish.


## How to run a Go WASM binary on a Bacalhau network

The Bacalhau documentation contains examples of running Docker containers or Rust programs as WASM, but here I want to focus on writing a WASM binary for Bacalhau in Go.

For the sake of simplicity, I will use a single, local Bacalhau server, so that you need nothing but a computer, the [Bacalhau CLI command][Bacalhau CLI], and [TinyGo][TinyGo], to follow the steps and create your own mini data processing system.


### Step 1: Write a Go program that counts words in files

The data processing job is quite simple: Read all files in a given directory and count the words. Return the per-file results in an output file, and send the total count of all files to `stdout`. 

The Go code is unspectacular. It does not need to know anything about Bacalhau. There are no packages to import. Bacalhau provides the job transparently with input and output directories and also collects everything the job writes to `stdout` and `stderr`. 

Here is the full code in all its boringness.

*/

// ## Imports and globals
func main() {
    // Bacalhau can map data sources to a "virtual" input directory. The path is arbitrary; we use `/inputs` here.
	inputDir := "/inputs"

    // Output can go to a dedicated output directory, to `stdout`, and to `stderr`. 
	outputDir := "/outputs"

	dir, err := os.Open(inputDir)
	if err != nil {
        // Here, we make use of the fact that Bacalhau collects `stderr` output as well.
		log.Fatal(err)
	}
	defer dir.Close()

    // Get all files in `/inputs`. For simplicity, let's assume all of them are plain text files.
	entries, err := dir.Readdirnames(-1)
	if err != nil {
		log.Fatal(err)
	}
	if len(entries) == 0 {
		log.Fatal("No files found")
	}

    // Write the results to "count.txt".
	out, err := os.Create(filepath.Join(outputDir, "count.txt"))
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

    // Besides the word count of each file, we also want to know the overall count. This value is sent to `stdout`. 
	total := 0

    // Iterate over all files in `/inputs` and count the words in each file.
	for _, entry := range entries {
		f, err := os.Open(filepath.Join(inputDir, entry))
		if err != nil {
			log.Fatal(err)
		}
		r := bufio.NewReader(f)

		words, err := countWords(r)
		f.Close()
		if err != nil {
			log.Fatal(err)
		}
		total += words

        // File-specific counts go to counts.txt
		fmt.Fprintf(out, "%s has %d words\n", entry, words)
	}

    // The total count goes to `stdout`.
	fmt.Println("Total word count: ", total)

// `countWords` scans words from an input stream and counts them. The algorithm is simple and counts everything, including comment symbols, markdown heading markers, etc.
func countWords(r *bufio.Reader) (int, error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)

	wordCount := 0
	for scanner.Scan() {
		wordCount++
	}

	if err := scanner.Err(); err != nil {
        // EOF is expected in this context. 
		if err == io.EOF {
			return wordCount, nil
		}
        // Else return an error.
		return wordCount, fmt.Errorf("countWords: %w", err)
	}

	return wordCount, nil
}
}
/*

### Step 2: Compile the program to WASM with TinyGo

Now it gets more interesting. We want to compile the code to a WASM binary. To save space, I'll use [TinyGo][TinyGo], a "Go compiler for small spaces". TinyGo produces compact WebAssembly binaries, with a few [restrictions][TinyGo restrictions] on Go features.

With TinyGo, compiling Go code to WASM is as easy as calling

```sh
> tinygo build -o main.wasm -target=wasi main.go
```

The resulting `main.wasm` binary is ready to be sent to data nodes.


#### Troubleshooting: Go version mismatch

TinyGo might exit with this error message (the actual version number may differ): 

```
error: requires go version 1.18 through 1.21, got go1.22
```

TinyGo does not hurry to catch up with the latest Go version. To fix this error, you need to install a Go version supported by TinyGo.

The easiest way is to download an older Go and set a local alias that is valid in the current shell:

```sh
> go get go@1.21.7
> go1.21.7 download
> alias go go1.21.7
```

Then repeat the `tinygo build` command. 


### Step 3: Start a local Bacalhau server

This would be the time to set up a Bacalhau network of data nodes. I'll simulate this network by starting a single Bacalhau server locally.

```sh
> bacalhau serve \
  --node-type requester,compute \
  --allow-listed-local-paths '/path/to/local/directory/with/inputs' \
  --web-ui
```

This server starts with some special settings:

- The node type is both requester and compute. Bacalhau distinguishes between requester nodes that orchestrate job requests, and compute node that run the jobs. This minimal server setup combines both types into one hybrid node.
- As a security measure, a specific local path is whitelisted for serving as the input directory for jobs. 
- Finally, we ask the server to run a web UI that lists active nodes and a history of job runs.

For my test, I created three files in the `inputs` directory, with the creative names `file1.txt`, `file2.txt`, and `file3.txt`. 

A few seconds after starting, the server list a few environment variables to set, so that the CLI command knows how to reach the server. 

For convenience, the environment variables are also written to `~/.bacalhau/bacalhau.run`. In a new shell, set these variables by running

```sh
> source ~/.bacalhau/bacalhau.run
```

Now that the server is running, we can send our first job.


### Step 4: Send the job to the "network"

In the shell where you sourced the Bacalhau environment settings, cd into the directory containing the WASM binary and call:

```sh
> bacalhau wasm run -i file:///path/to/input/directory:/inputs main.wasm
```

Here, `/path/to/input/directory` is the full path to the directory containing the input files. After the colon, specify the path as the job would see it. The Go code looks into `/inputs`, and so the actual input path is mapped to `/inputs`. 

(Note that `/inputs` happens to be the default input directory, so unless your job binary expects a different input path, you can omit the `:/inputs` part from the `-i` parameter.)

If all goes well, the command should return an output similar to the following:

```
Job successfully submitted. Job ID: 113c9bc7-7f54-474d-bfe1-7839b32984cf
Checking job status... (Enter Ctrl+C to exit at any time, your job will continue running):

	Communicating with the network  ................  done ✅  0.0s
	   Creating job for submission  ................  done ✅  0.5s
	               Job in progress  ................  done ✅  0.0s

To download the results, execute:
	bacalhau get 113c9bc7-7f54-474d-bfe1-7839b32984cf

To get more details about the run, execute:
	bacalhau describe 113c9bc7-7f54-474d-bfe1-7839b32984cf
```


### Step 5: Collect the data

The outputs of the job get a unique ID assigned. To "download" the results (let's pretend the server runs on a remote node), run the provided `get` subcommand:

```
> bacalhau get dc3b187f-de18-4b38-a1e1-e2f20172d8ef
Fetching results of job 'dc3b187f-de18-4b38-a1e1-e2f20172d8ef'...
Results for job 'dc3b187f-de18-4b38-a1e1-e2f20172d8ef' have been written to...
/path/to/job/results/job-dc3b187f
```

Each job has a unique results directory that contains 

- the output directory,
- and the files `stdout`, `stderr`, and `exitCode`.

Yes, Bacalhau collects everything that the job would return if it ran in a normal shell: file output, shell output and the exit code. 

Read the files to verify that the job did its job (apologies for the pun):

```sh
> cat job-dc3b187f/stdout
Total word count:  414
```

```sh
 > cat job-dc3b187f/outputs/count.txt                                                                  0s
file2.txt has 138 words
file3.txt has 207 words
file1.txt has 69 words
```


## Summing up: Bacalhau is distributed computing without the orchestration overhead

A few commands were sufficient to set up a single-node Compute-over-Data network. With a few more commands, you can set up a real, distributed network of Bacalhau nodes and send them compute jobs. 

You can be specific about the nodes to use, such as: "Send this job only to nodes that have a GPU". Jobs can fetch data from local directories, URLs, S3 buckets, or IPFS networks, and send output to any of these locations except URLs. Nodes can join and leave the network, and the client will always know which nodes are available and which nodes to send a given job to. 

Bacalhau is a powerful system for distributed data processing, yet easy to set up and use. And Go is the perfect language for compute job payloads. Go apps can live in small containers or WebAssembly modules, saving bandwidth when distributing jobs. 

Maybe I'll set up a real Bacalhau cluster next.

## Links

- [Bacalhau][Bacalhau]
- [TinyGo][TinyGo]


[Bacalhau]: https://bacalhau.org
[Bacalhau CLI]: https://docs.bacalhau.org/getting-started/installation
[TinyGo]: https://tinygo.org
[TinyGo restrictions]: https://tinygo.org/docs/reference/lang-support/


**Happy coding!**

___

<small>Cod photo by <a href="https://unsplash.com/@rresenden?utm_content=creditCopyText&utm_medium=referral&utm_source=unsplash">Ricardo Resende</a> on <a href="https://unsplash.com/photos/white-fish-mlzRoZqv_zM?utm_content=creditCopyText&utm_medium=referral&utm_source=unsplash">Unsplash</a>
  </small>

*/
