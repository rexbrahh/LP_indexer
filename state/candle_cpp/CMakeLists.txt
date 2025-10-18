cmake_minimum_required(VERSION 3.20)
project(candle_cpp CXX)

include(FetchContent)

get_filename_component(REPO_ROOT "${CMAKE_SOURCE_DIR}/../.." REALPATH)
set(PROTO_GEN_CPP_DIR "${REPO_ROOT}/gen/cpp")

find_package(Protobuf CONFIG REQUIRED)
find_package(absl CONFIG REQUIRED)

set(NATS_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(NATS_BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(NATS_BUILD_CLIENT_UTILS OFF CACHE BOOL "" FORCE)
set(NATS_BUILD_TLS ON CACHE BOOL "" FORCE)

FetchContent_Declare(
  natsc
  GIT_REPOSITORY https://github.com/nats-io/nats.c.git
  GIT_TAG v3.6.1
)
FetchContent_MakeAvailable(natsc)

# C++20 standard
set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# Compiler flags
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wall -Wextra -Wpedantic")
set(CMAKE_CXX_FLAGS_DEBUG "${CMAKE_CXX_FLAGS_DEBUG} -g -O0")
set(CMAKE_CXX_FLAGS_RELEASE "${CMAKE_CXX_FLAGS_RELEASE} -O3")

# Source files
set(CANDLE_SOURCES
    src/fixed_point.cpp
    src/candle_worker.cpp
    src/publisher.cpp
    ${PROTO_GEN_CPP_DIR}/dex/sol/v1/core.pb.cc
)

# Create library
add_library(candle_cpp STATIC ${CANDLE_SOURCES})
target_include_directories(candle_cpp
  PUBLIC
    include
    ${PROTO_GEN_CPP_DIR}
  PRIVATE
    ${natsc_SOURCE_DIR}/src
)
target_link_libraries(candle_cpp PUBLIC protobuf::libprotobuf absl::strings nats_static)

# Fetch GoogleTest
FetchContent_Declare(
  googletest
  GIT_REPOSITORY https://github.com/google/googletest.git
  GIT_TAG v1.14.0
)
# For Windows: Prevent overriding the parent project's compiler/linker settings
set(gtest_force_shared_crt ON CACHE BOOL "" FORCE)
FetchContent_MakeAvailable(googletest)

# Enable testing
enable_testing()

# Fixed point test
add_executable(fixed_point_test tests/fixed_point_test.cpp)
target_link_libraries(fixed_point_test candle_cpp GTest::gtest GTest::gtest_main pthread)
add_test(NAME fixed_point_test COMMAND fixed_point_test)

# Candle finalization test
add_executable(candle_finalize_test tests/candle_finalize_test.cpp)
target_link_libraries(candle_finalize_test candle_cpp GTest::gtest GTest::gtest_main pthread)
add_test(NAME candle_finalize_test COMMAND candle_finalize_test)

add_executable(candle_publisher_dispatch_test tests/publisher_dispatch_test.cpp)
target_link_libraries(candle_publisher_dispatch_test candle_cpp GTest::gtest GTest::gtest_main pthread)
add_test(NAME candle_publisher_dispatch_test COMMAND candle_publisher_dispatch_test)

add_executable(candle_replay tools/candle_replay.cpp)
target_link_libraries(candle_replay candle_cpp protobuf::libprotobuf nats_static)
