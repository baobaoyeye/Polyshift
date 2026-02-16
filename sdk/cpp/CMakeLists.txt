cmake_minimum_required(VERSION 3.15)
project(polyshift-cpp-sdk)

set(CMAKE_CXX_STANDARD 17)

find_package(Protobuf CONFIG REQUIRED)
find_package(gRPC CONFIG REQUIRED)

include_directories(${Protobuf_INCLUDE_DIRS})
include_directories(${CMAKE_CURRENT_SOURCE_DIR})
include_directories(${CMAKE_CURRENT_SOURCE_DIR}/include)

set(PROTO_SRCS
    proto/plugin/plugin.pb.cc
    proto/plugin/plugin.grpc.pb.cc
)

set(SDK_SRCS
    src/server.cc
)

add_library(polyshift-cpp-sdk STATIC
    ${PROTO_SRCS}
    ${SDK_SRCS}
)

target_link_libraries(polyshift-cpp-sdk
    gRPC::grpc++
    protobuf::libprotobuf
)

target_include_directories(polyshift-cpp-sdk PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/include
    ${CMAKE_CURRENT_SOURCE_DIR}
)
