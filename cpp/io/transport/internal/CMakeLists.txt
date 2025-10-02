message(STATUS "Building transport internal library")

add_library(io_transport_internal OBJECT
  callmap.cc
)

target_include_directories(io_transport_internal PUBLIC ${CMAKE_SOURCE_DIR})
