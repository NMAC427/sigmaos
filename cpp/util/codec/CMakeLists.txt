message(STATUS "Building codec library")

add_library(util_codec OBJECT codec.cc)

target_include_directories(util_codec PUBLIC ${CMAKE_SOURCE_DIR})
