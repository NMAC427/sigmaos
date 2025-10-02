message(STATUS "Building tracing proto library")

add_library(util_tracing_proto OBJECT tracing.pb.cc)

target_link_libraries(util_tracing_proto PUBLIC ${Protobuf_LIBRARIES})

target_include_directories(util_tracing_proto PUBLIC
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
