message(STATUS "Building sigmap proxy proto library")

add_library(sigmap_proxy_proto STATIC spproxy.pb.cc)

target_link_libraries(sigmap_proxy_proto PUBLIC
  /usr/lib/libprotobuf.so
  rpc_proto
  proc_proto
)

target_include_directories(sigmap_proxy_proto PUBLIC 
  ${Protobuf_INCLUDE_DIRS}
  ${CMAKE_SOURCE_DIR}
)
