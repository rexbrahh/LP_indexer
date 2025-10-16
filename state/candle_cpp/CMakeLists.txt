cmake_minimum_required(VERSION 3.20)
project(candle_cpp CXX)

# C++20 standard
set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# Compiler flags
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wall -Wextra -Wpedantic")
set(CMAKE_CXX_FLAGS_DEBUG "${CMAKE_CXX_FLAGS_DEBUG} -g -O0")
set(CMAKE_CXX_FLAGS_RELEASE "${CMAKE_CXX_FLAGS_RELEASE} -O3")

# Include directories
include_directories(include)

# Source files
set(CANDLE_SOURCES
    src/fixed_point.cpp
    src/candle_worker.cpp
    src/publisher.cpp
)

# Create library
add_library(candle_cpp STATIC ${CANDLE_SOURCES})

# Fetch GoogleTest
include(FetchContent)
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
