message(STATUS "Building epcache proto library")

add_library(epcache_proto OBJECT epcache.pb.cc)

target_include_directories(epcache_proto PUBLIC
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
