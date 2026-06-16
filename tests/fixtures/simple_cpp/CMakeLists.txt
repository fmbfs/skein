cmake_minimum_required(VERSION 3.20)
project(simple_cpp LANGUAGES CXX)
set(CMAKE_CXX_STANDARD 20)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)
add_executable(simple
    src/main.cpp
    src/pipeline.cpp
)
target_include_directories(simple PRIVATE include)
