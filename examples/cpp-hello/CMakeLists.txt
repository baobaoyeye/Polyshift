cmake_minimum_required(VERSION 3.15)
project(cpp-hello)

set(CMAKE_CXX_STANDARD 17)

find_package(Protobuf CONFIG REQUIRED)
find_package(gRPC CONFIG REQUIRED)

# Add SDK
add_subdirectory(../../sdk/cpp sdk_build)

add_executable(cpp-hello main.cc)

target_link_libraries(cpp-hello
    polyshift-cpp-sdk
    gRPC::grpc++
    protobuf::libprotobuf
)
