message(STATUS "Building rpc proto library")

add_library(rpc_proto OBJECT rpc.pb.cc)

target_include_directories(rpc_proto PUBLIC
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
