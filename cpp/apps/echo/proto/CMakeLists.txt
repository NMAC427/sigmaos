message(STATUS "Building echo proto library")

add_library(echo_proto OBJECT example_echo_server.pb.cc)

target_include_directories(echo_proto PUBLIC
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
