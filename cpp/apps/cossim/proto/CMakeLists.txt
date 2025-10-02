message(STATUS "Building cossim proto library")

add_library(cossim_proto OBJECT cossim.pb.cc)

target_include_directories(cossim_proto PUBLIC
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
