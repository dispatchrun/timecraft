FROM ubuntu:22.10

RUN apt-get update && apt-get install -y clang-15 llvm-15 lld-15 make wget software-properties-common zip git
RUN add-apt-repository ppa:deadsnakes/ppa \
 && apt-get install -y python3.11 python3.11-venv
RUN wget https://bootstrap.pypa.io/get-pip.py \
 && python3.11 get-pip.py
RUN wget https://github.com/WebAssembly/binaryen/releases/download/version_113/binaryen-version_113-x86_64-linux.tar.gz \
 && tar xvf binaryen-version_113-x86_64-linux.tar.gz \
 && cp ./binaryen-version_113/bin/wasm-opt /bin/wasm-opt \
 && rm -rf binaryen-version_113-x86_64-linux.tar.gz ./binaryen-version_113
