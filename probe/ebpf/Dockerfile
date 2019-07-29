FROM fedora:30
RUN dnf install -y kernel-devel make binutils golang go-bindata git cmake gcc g++

RUN git clone -b release_90 https://git.llvm.org/git/llvm.git ; pushd llvm/tools ; git clone -b release_90 https://git.llvm.org/git/clang.git ; popd ; pushd llvm ; mkdir build; cd build ; cmake .. -DLLVM_TARGETS_TO_BUILD="BPF;X86" -DBUILD_SHARED_LIBS=OFF -DCMAKE_BUILD_TYPE=Release -DLLVM_BUILD_RUNTIME=OFF ; make -j $(getconf _NPROCESSORS_ONLN); make install ; popd ; rm -rf llvm
