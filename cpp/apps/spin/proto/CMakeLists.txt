message(STATUS "Building spin proto library")

add_library(spin_proto OBJECT spin.pb.cc)

target_include_directories(spin_proto PUBLIC
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
