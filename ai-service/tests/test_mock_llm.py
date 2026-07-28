from app.testing.mock_llm import _current_question


def test_mock_provider_reads_rag_and_agent_question_formats():
    assert _current_question([{"role": "user", "content": "QUESTION=测试超时\n"}]) == "测试超时"
    assert (
        _current_question([{"role": "user", "content": '{"question":"夜景怎么拍？"}'}])
        == "夜景怎么拍？"
    )
