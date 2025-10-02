cmake_minimum_required(VERSION 3.28)
project(sigmaos CXX)

set(CMAKE_CXX_STANDARD 23)
set(CMAKE_CXX_STANDARD_REQUIRED True)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

set(CMAKE_POSITION_INDEPENDENT_CODE ON)

if(NOT CMAKE_BUILD_TYPE)
  set(CMAKE_BUILD_TYPE Release)
endif()

# Enable ccache if available
find_program(CCACHE_PROGRAM ccache)
if(CCACHE_PROGRAM)
  set(CMAKE_CXX_COMPILER_LAUNCHER "${CCACHE_PROGRAM}")
  set(CMAKE_C_COMPILER_LAUNCHER "${CCACHE_PROGRAM}")
  message(STATUS "Using ccache: ${CCACHE_PROGRAM}")
endif()

# External packages
find_package(Protobuf REQUIRED)
find_package(absl REQUIRED)
find_package(spdlog REQUIRED)

find_package(Python 3.11 EXACT COMPONENTS Interpreter Development REQUIRED)
find_package(pybind11 CONFIG REQUIRED)

# backward-cpp provides pretty backtraces for segfaults and other crashes
include(FetchContent)
FetchContent_Declare(backward
    GIT_REPOSITORY https://github.com/bombela/backward-cpp
    GIT_TAG master
    SYSTEM
)
FetchContent_MakeAvailable(backward)

# SigmaOS packages
add_subdirectory(serr)
add_subdirectory(io)
add_subdirectory(util)
add_subdirectory(proc)
add_subdirectory(proxy)
add_subdirectory(python)
add_subdirectory(sigmap)
add_subdirectory(rpc)
add_subdirectory(threadpool)

# TODO: Maybe it would be worth turning this into a shared library?
message(STATUS "Building sigmaos core library")
add_library(sigmaos_core STATIC
  $<TARGET_OBJECTS:serr>
  $<TARGET_OBJECTS:util_common>
  $<TARGET_OBJECTS:util_codec>
  $<TARGET_OBJECTS:util_log>
  $<TARGET_OBJECTS:util_perf>
  $<TARGET_OBJECTS:util_tracing_proto>
  $<TARGET_OBJECTS:threadpool>
  $<TARGET_OBJECTS:io_iovec>
  $<TARGET_OBJECTS:io_frame>
  $<TARGET_OBJECTS:io_conn>
  $<TARGET_OBJECTS:io_conn_tcp>
  $<TARGET_OBJECTS:io_conn_unix>
  $<TARGET_OBJECTS:io_transport_internal>
  $<TARGET_OBJECTS:io_transport>
  $<TARGET_OBJECTS:io_demux_internal>
  $<TARGET_OBJECTS:io_demux>
  $<TARGET_OBJECTS:io_net>
  $<TARGET_OBJECTS:proc>
  $<TARGET_OBJECTS:proc_proto>
  $<TARGET_OBJECTS:rpc_proto>
  $<TARGET_OBJECTS:rpc_delegation>
  $<TARGET_OBJECTS:rpc_spchannel>
  $<TARGET_OBJECTS:rpc>
  $<TARGET_OBJECTS:sigmap_proto>
  $<TARGET_OBJECTS:sigmap_proxy_proto>
  $<TARGET_OBJECTS:sigmap_proxy>
)

target_link_libraries(sigmaos_core PUBLIC
  ${Protobuf_LIBRARIES}
  spdlog::spdlog
  absl::hash
  absl::status
)

target_include_directories(sigmaos_core PUBLIC ${CMAKE_SOURCE_DIR})

# User procs
add_subdirectory(apps)
add_subdirectory(user)
