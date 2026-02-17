cmake_minimum_required(VERSION 3.15)
project(polyshift-cpp-sdk)

set(CMAKE_CXX_STANDARD 17)

find_package(Protobuf CONFIG REQUIRED)
find_package(OpenSSL REQUIRED)
if(NOT TARGET OpenSSL::SSL)
    add_library(OpenSSL::SSL UNKNOWN IMPORTED)
    set_target_properties(OpenSSL::SSL PROPERTIES
        IMPORTED_LOCATION "${OPENSSL_SSL_LIBRARY}"
        INTERFACE_INCLUDE_DIRECTORIES "${OPENSSL_INCLUDE_DIR}"
    )
    add_library(OpenSSL::Crypto UNKNOWN IMPORTED)
    set_target_properties(OpenSSL::Crypto PROPERTIES
        IMPORTED_LOCATION "${OPENSSL_CRYPTO_LIBRARY}"
        INTERFACE_INCLUDE_DIRECTORIES "${OPENSSL_INCLUDE_DIR}"
    )
endif()

find_package(gRPC CONFIG REQUIRED)
find_package(opentelemetry-cpp CONFIG REQUIRED)
find_package(GTest CONFIG REQUIRED)

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

target_link_libraries(polyshift-cpp-sdk PUBLIC
    gRPC::grpc++
    protobuf::libprotobuf
    opentelemetry-cpp::trace
    opentelemetry-cpp::otlp_grpc_exporter
    opentelemetry-cpp::ostream_span_exporter
    opentelemetry-cpp::common
    opentelemetry-cpp::resources
)

target_include_directories(polyshift-cpp-sdk PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/include
    ${CMAKE_CURRENT_SOURCE_DIR}
)

add_executable(polyshift-cpp-plugin src/main.cc)
target_link_libraries(polyshift-cpp-plugin
    polyshift-cpp-sdk
)

enable_testing()

add_executable(tracing_test test/tracing_test.cc)
target_link_libraries(tracing_test
    polyshift-cpp-sdk
    GTest::gtest_main
    opentelemetry-cpp::trace
    opentelemetry-cpp::sdk
)

add_test(NAME tracing_test COMMAND tracing_test)
